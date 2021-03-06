/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package istio

import (
	"fmt"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/kubernetes"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"istio.io/api/networking/v1alpha3"
	istioNetworking "istio.io/client-go/pkg/apis/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubernetes2 "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	IstioLabelInjection = "istio-injection"
	InstPrefixLength    = 6
	OrgPrefixLength     = 8
)

type IstioDecorator struct {
	// Istio client
	Client *versionedclient.Clientset
	// Regular K8s client
	KClient *kubernetes2.Clientset
}

func NewIstioDecorator() (executor.NetworkDecorator, derrors.Error) {

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, derrors.NewInternalError("impossible to get local configuration for internal k8s client", err)
	}

	// Get a versioned Istio client
	ic, err := versionedclient.NewForConfig(config)
	if err != nil {
		log.Error().Err(err).Msg("impossible to build a local Istio client")
		return nil, derrors.NewInternalError("impossible to build a local Istio client", err)
	}

	// Get a service client
	sc, err := kubernetes2.NewForConfig(config)
	if err != nil {
		log.Error().Err(err).Msg("impossible to build a local k8s service interface client")
		return nil, derrors.NewInvalidArgumentError("impossible to build a local service interface client", err)
	}

	return &IstioDecorator{Client: ic, KClient: sc}, nil

}

func (id *IstioDecorator) Build(aux executor.Deployable, args ...interface{}) derrors.Error {
	switch target := aux.(type) {
	// Process a namespace
	case *kubernetes.DeployableNamespace:
		return id.decorateNamespace(target)
	case *kubernetes.DeployableDeployments:
		return id.decorateDeployments(target)
	default:
		// nothing to do
		return nil
	}
	return nil
}

func (id *IstioDecorator) Deploy(aux executor.Deployable, args ...interface{}) derrors.Error {

	switch target := aux.(type) {
	case *kubernetes.DeployableServices:
		return id.decorateServices(target)
	}
	return nil
}

// Remove any unnecessary entries when a deployable element is removed.
func (id *IstioDecorator) Undeploy(aux executor.Deployable, args ...interface{}) derrors.Error {
	return nil
}

// Decorate services by extending the number of available services to include those services
// that are declared to be accessible but are not deployed onto this cluster.
// params:
//  target service to be decorated
// return:
//  error if any
func (id *IstioDecorator) decorateServices(target *kubernetes.DeployableServices) derrors.Error {

	// Create a service for every rule allowing internal traffic if it is not declared yet.
	for _, publicRule := range target.Data.Stage.PublicRules {

		found := false
		for _, s := range target.Services {
			// If we already have a service for this public rule skip to the next one
			if publicRule.ServiceName == s.Service.Name {
				found = true
				break
			}
		}

		log.Debug().Str("serviceName", publicRule.ServiceName).Msgf("comparing existing services we have found it %t", found)
		if found {
			continue
		}

		// Try to get the service and if it is there add the port for this service if not available.
		foundServ, errServ := id.KClient.CoreV1().Services(target.Data.Namespace).
			Get(common.FormatName(publicRule.ServiceName), metaV1.GetOptions{})
		if errors.IsNotFound(errServ) {
			log.Debug().Str("serviceName", common.FormatName(publicRule.ServiceName)).
				Msg("service does not exists. Create it")
			// Service not found for this rule, create one
			newServ := apiv1.Service{
				ObjectMeta: metaV1.ObjectMeta{
					Name:      common.FormatName(publicRule.ServiceName),
					Namespace: target.Data.Namespace,
					Labels: map[string]string{
						utils.NALEJ_ANNOTATION_ORGANIZATION_ID: target.Data.OrganizationId,
						utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:  target.Data.AppDescriptorId,
						utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: target.Data.AppInstanceId,
						utils.NALEJ_ANNOTATION_IS_PROXY:        "false",
					},
				},
				Spec: apiv1.ServiceSpec{
					ExternalName: common.FormatName(publicRule.ServiceName),
					Ports: []apiv1.ServicePort{
						{
							Port: publicRule.TargetPort,
							// TODO we have to assume that the internal port matches
							TargetPort: intstr.IntOrString{IntVal: publicRule.TargetPort},
							Name:       fmt.Sprintf("port%d", publicRule.TargetPort),
						},
					},
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: target.Data.AppInstanceId,
						utils.NALEJ_ANNOTATION_ORGANIZATION_ID: target.Data.OrganizationId,
						utils.NALEJ_ANNOTATION_SERVICE_NAME:    common.FormatName(publicRule.ServiceName),
					},
				},
			}
			_, errCreateServ := id.KClient.CoreV1().Services(target.Data.Namespace).Create(&newServ)
			if errCreateServ != nil {
				log.Error().Err(errCreateServ).Msg("error creating service from Istio decorator")
			}
		} else {
			log.Debug().Str("serviceName", common.FormatName(publicRule.ServiceName)).
				Msg("service already exists. Update it")
			updateServiceErr := id.updateServicePorts(foundServ, publicRule)
			if updateServiceErr != nil {
				log.Error().Err(updateServiceErr).
					Msg("there was an error updating an existing service by istio decorator")
				return updateServiceErr
			}
		}

		// Similarly, create a virtual service if it corresponds
		// update the virtual service
		foundVirtualServ, virtualServErr := id.Client.NetworkingV1alpha3().
			VirtualServices(target.Data.Namespace).Get(common.FormatName(publicRule.ServiceName), metaV1.GetOptions{})
		if errors.IsNotFound(virtualServErr) {
			// we have to create the virtual service
			// Create a virtual service to redirect this
			vs := &istioNetworking.VirtualService{
				ObjectMeta: metaV1.ObjectMeta{
					Name: common.FormatName(publicRule.ServiceName),
					Namespace: target.Data.Namespace,
				},
				Spec: v1alpha3.VirtualService{
					Hosts: []string{publicRule.ServiceName},
					Tcp: []*v1alpha3.TCPRoute{
						{
							Route: []*v1alpha3.RouteDestination{
								{
									Destination: &v1alpha3.Destination{
										Host: common.FormatName(publicRule.ServiceName),
										Port: &v1alpha3.PortSelector{Number: uint32(publicRule.TargetPort)}},
								},
							},
							Match: []*v1alpha3.L4MatchAttributes {
								{
									Port: uint32(publicRule.TargetPort),
								},
							},
						},
					},
				},
			}
			log.Debug().Msg("create virtual service")
			_, errVS := id.Client.NetworkingV1alpha3().VirtualServices(target.Data.Namespace).Create(vs)
			if errVS != nil {
				return derrors.NewInternalError("impossible to generate virtual service", errVS)
			}
		} else {
			updateVirtualServiceErr := id.updateVirtualServicePorts(foundVirtualServ, publicRule)
			if updateVirtualServiceErr != nil {
				log.Error().Err(updateVirtualServiceErr).
					Msg("there was an error updating a virtual service error by istio decorator")
				return derrors.NewInternalError("there was an error updating a virtual service error by istio decorator", virtualServErr)
			}
		}
	}

	return nil
}

// Decorate deployments to skip Istio network catching.
// params:
//  target kubernetes deployment to be decorated
// return:
//   error if any
func (id *IstioDecorator) decorateDeployments(target *kubernetes.DeployableDeployments) derrors.Error {
	// Those services connected with the ingress we have to disable the inbound ports
	for _, publicRule := range target.Data.Stage.PublicRules {
		for _, dep := range target.Deployments {
			if publicRule.TargetServiceGroupInstanceId == dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] &&
				publicRule.TargetServiceInstanceId == dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] {
				log.Debug().Str("serviceName", dep.Name).Msg("candidate for Istio ingress endpoint")
				// Set the corresponding flags
				if dep.Spec.Template.Annotations == nil {
					dep.Spec.Template.Annotations = make(map[string]string, 0)
				}
				//TODO services exposed using the Nginx ingress + istio are assumed to only receive incoming traffic
				//     from the ingress. Other services under the Istio network umbrella will be ignored.
				dep.Spec.Template.Annotations["traffic.sidecar.istio.io/includeInboundPorts"] = ""
			}
		}
	}

	return nil
}

// Add the Istio labels required by any namespace to enable de network traffic injection.
// params:
//  namespace to be modified
// return:
//  error if any
func (id *IstioDecorator) decorateNamespace(namespace *kubernetes.DeployableNamespace) derrors.Error {
	namespace.Namespace.Labels[IstioLabelInjection] = "enabled"
	return nil
}

func (id *IstioDecorator) generateLabels(d *kubernetes.DeployableIngress,
	service *grpc_application_go.ServiceInstance) map[string]string {

	labels := make(map[string]string, 0)
	if service.Labels != nil {
		// users have already defined labels for this app
		for k, v := range service.Labels {
			labels[k] = v
		}
	}

	labels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = d.Data.OrganizationId
	labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.Data.AppDescriptorId
	labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.Data.AppInstanceId
	labels[utils.NALEJ_ANNOTATION_IS_PROXY] = "false"

	return labels
}

func (id *IstioDecorator) getNamePrefixes(service *grpc_application_go.ServiceInstance,
	rule *grpc_conductor_go.PublicSecurityRuleInstance) (string, string, string, string) {
	ingressName := service.Name
	if rule.TargetPort != 80 {
		ingressName = fmt.Sprintf("%s-%d", service.Name, rule.TargetPort)
	}
	serviceGroupInstPrefix := service.ServiceGroupInstanceId
	if len(serviceGroupInstPrefix) > InstPrefixLength {
		serviceGroupInstPrefix = serviceGroupInstPrefix[0:InstPrefixLength]
	}
	appInstPrefix := service.AppInstanceId
	if len(appInstPrefix) > InstPrefixLength {
		appInstPrefix = appInstPrefix[0:InstPrefixLength]
	}
	orgPrefix := service.OrganizationId
	if len(orgPrefix) > OrgPrefixLength {
		orgPrefix = orgPrefix[0:OrgPrefixLength]
	}
	return ingressName, serviceGroupInstPrefix, appInstPrefix, orgPrefix
}

// This function updates the definition of an existing port with the ports defined in an existing service and
// tries to apply the changes to the current service.
// params:
//  service to be updated
//  public rule with rules
// return:
//  error if any
func (id *IstioDecorator) updateServicePorts(service *apiv1.Service, rule *grpc_conductor_go.PublicSecurityRuleInstance) derrors.Error{
	// check if the current service has this port
	found := false
	for _, p := range service.Spec.Ports {
		if p.Port == rule.TargetPort {
			found = true
			break
		}
	}

	if !found {
		service.Spec.Ports = append(service.Spec.Ports, apiv1.ServicePort{
			Port: rule.TargetPort,
			// TODO we have to assume that the internal port matches
			TargetPort: intstr.IntOrString{IntVal: rule.TargetPort},
			Name: fmt.Sprintf("port%d",rule.TargetPort),
		})
		// update it

		_, err := id.KClient.CoreV1().Services(service.Namespace).Update(service)
		if err != nil {
			log.Error().Err(err).Msg("impossible to update service by istio decorator")
			return derrors.NewInternalError("impossible to update service by istio decorator", err)
		}
	}

	return nil
}

// Update an existing virtual service by adding the ports indicated in the public security rule only in case they are
// not available there yet.
// params:
//  virtualService the entitity to be updated
//  rule with the ports to be added to the service
// return:
//  error if any
func (id *IstioDecorator) updateVirtualServicePorts(virtualService *istioNetworking.VirtualService,
	rule *grpc_conductor_go.PublicSecurityRuleInstance) derrors.Error {

	// Check if the virtual service has already this port
	found := false
	for _, tcpRule := range virtualService.Spec.Tcp{
		if tcpRule.Match[0].Port == uint32(rule.TargetPort) {
			found = true
		}
	}
	if found{
		return nil
	}

	// create a new tcp rule
	newRoute := v1alpha3.TCPRoute{
			Route: []*v1alpha3.RouteDestination{
			{
				Destination: &v1alpha3.Destination{
					Host: common.FormatName(rule.ServiceName),
					Port: &v1alpha3.PortSelector{Number: uint32(rule.TargetPort)}},
			},
		},
			Match: []*v1alpha3.L4MatchAttributes {
				{
				Port: uint32(rule.TargetPort),
				},
			},
		}
	virtualService.Spec.Tcp = append(virtualService.Spec.Tcp, &newRoute)

	_, err := id.Client.NetworkingV1alpha3().VirtualServices(virtualService.ObjectMeta.Namespace).Update(virtualService)
	if err != nil {
		log.Error().Err(err).Interface("virtualService", virtualService).Msg("istio decorator found a problem when updating virtual service")
		return derrors.NewInternalError("istio decorator found a problem when updating virtual service", err)
	}

	return nil
}