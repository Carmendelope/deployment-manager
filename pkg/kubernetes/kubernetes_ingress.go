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
 */

package kubernetes

import (
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/extensions/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	extV1Beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

const InstPrefixLength = 6
const OrgPrefixLength = 8

const ANNOTATION_PATH = "nginx.ingress.kubernetes.io/app-root"

type IngressesInfo struct {
	ServiceId         string
	ServiceInstanceId string
	Ingresses         []*v1beta1.Ingress
}

type DeployableIngress struct {
	client    extV1Beta1.IngressInterface
	Data      entities.DeploymentMetadata
	Ingresses []IngressesInfo
	// network decorator object for deployments
	networkDecorator executor.NetworkDecorator
}

func NewDeployableIngress(
	client *kubernetes.Clientset,
	data entities.DeploymentMetadata, networkDecorator executor.NetworkDecorator) *DeployableIngress {
	return &DeployableIngress{
		client:           client.ExtensionsV1beta1().Ingresses(data.Namespace),
		Data:             data,
		Ingresses:        make([]IngressesInfo, 0),
		networkDecorator: networkDecorator,
	}
}

func (di *DeployableIngress) GetId() string {
	return di.Data.Stage.StageId
}

func (di *DeployableIngress) GetIngressesEndpoints() map[string][]string {
	result := make(map[string][]string, 0)

	for _, ings := range di.Ingresses {
		endpoints := make([]string, 0)
		for _, endpoint := range ings.Ingresses {
			endpoints = append(endpoints, endpoint.Spec.Rules[0].Host)
		}
		result[ings.ServiceId] = endpoints
	}

	return result
}

func (di *DeployableIngress) getNamePrefixes(service *grpc_application_go.ServiceInstance, rule *grpc_conductor_go.PublicSecurityRuleInstance) (string, string, string, string) {
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
	orgPrefix := di.Data.OrganizationId
	if len(orgPrefix) > OrgPrefixLength {
		orgPrefix = orgPrefix[0:OrgPrefixLength]
	}
	return ingressName, serviceGroupInstPrefix, appInstPrefix, orgPrefix
}

func (di *DeployableIngress) BuildIngressesForServiceWithRule(service *grpc_application_go.ServiceInstance, rule *grpc_conductor_go.PublicSecurityRuleInstance) *v1beta1.Ingress {

	paths := make([]v1beta1.HTTPIngressPath, 0)

	annotationPath := ""

	found := false
	for portIndex := 0; portIndex < len(service.ExposedPorts) && !found; portIndex++ {
		port := service.ExposedPorts[portIndex]
		if port.ExposedPort == rule.TargetPort && port.Endpoints != nil {
			for endpointIndex := 0; endpointIndex < len(service.ExposedPorts[portIndex].Endpoints) && !found; endpointIndex++ {
				endpoint := service.ExposedPorts[portIndex].Endpoints[endpointIndex]
				if endpoint.Type == grpc_application_go.EndpointType_WEB || endpoint.Type == grpc_application_go.EndpointType_REST {
					if endpoint.Path != "/" {
						annotationPath = endpoint.Path
					}
					toAdd := v1beta1.HTTPIngressPath{
						Backend: v1beta1.IngressBackend{
							ServiceName: service.Name,
							ServicePort: intstr.IntOrString{IntVal: rule.TargetPort},
						},
					}
					paths = append(paths, toAdd)
					found = true
				}
			}
		}
	}

	if !found {
		log.Warn().Str("serviceId", service.ServiceId).Msg("rule mismatch for ingress definition")
		return nil
	}

	if len(paths) == 0 {
		log.Debug().Str("serviceId", service.ServiceId).Msg("service does not contain any paths")
		return nil
	}

	ingressName, serviceGroupInstPrefix, appInstPrefix, orgPrefix := di.getNamePrefixes(service, rule)

	// labels cannot be higher than 63 characters
	ingressPrefixName := fmt.Sprintf("%s.%s.%s", ingressName, serviceGroupInstPrefix, appInstPrefix)
	ingressGlobalFqdn := fmt.Sprintf("%s.%s.%s.%s.ep.%s", ingressName, serviceGroupInstPrefix, appInstPrefix, orgPrefix, config.GetConfig().ManagementHostname)
	ingressHostname := fmt.Sprintf("%s.%s.%s.appcluster.%s", ingressName, serviceGroupInstPrefix, appInstPrefix, config.GetConfig().ClusterPublicHostname)

	// create the ingress annotations
	annotations := map[string]string{
		"kubernetes.io/ingress.class": "nginx",
		"nginx.ingress.kubernetes.io/service-upstream": "true",
		"organizationId":              service.OrganizationId,
		"appInstanceId":               di.Data.AppInstanceId,
		"serviceId":                   service.ServiceId,
	}
	// Overwrite the application root path with the user specified one so that when
	// the user accesses the endpoint through the DNS it is automatically redirected
	if annotationPath != "" {
		annotations[ANNOTATION_PATH] = annotationPath
	}

	return &v1beta1.Ingress{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      fmt.Sprintf("ingress-%s-%d", service.ServiceId, rule.TargetPort),
			Namespace: di.Data.Namespace,
			Labels: map[string]string{
				"cluster":   "application",
				"component": "ingress-nginx",
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT:       di.Data.FragmentId,
				utils.NALEJ_ANNOTATION_INGRESS_ENDPOINT:          ingressPrefixName,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:           di.Data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:            di.Data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:           di.Data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:                  di.Data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:                service.ServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID:       service.ServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID:          service.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID: service.ServiceGroupInstanceId,
			},
			Annotations: annotations,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				// Ingress hostname with the DNS entry pointing to the application cluster.
				{
					Host: ingressHostname,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
				// Ingress hostname with the DNS entry pointing to the global fqdn.
				{
					Host: ingressGlobalFqdn,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
			},
		},
	}
}

// TODO Check the rules to build the Ingresses.
func (di *DeployableIngress) Build() error {
	log.Debug().Int("number public rules", len(di.Data.Stage.PublicRules)).Msg("Building Ingresses")

	for _, publicRule := range di.Data.Stage.PublicRules {
		log.Debug().Interface("rule", publicRule).Msg("Checking public rule")
		for _, service := range di.Data.Stage.Services {
			log.Debug().Interface("service", service).Msg("Checking service for public ingress")
			if publicRule.TargetServiceGroupInstanceId == service.ServiceGroupInstanceId && publicRule.TargetServiceInstanceId == service.ServiceInstanceId {
				toAdd := di.BuildIngressesForServiceWithRule(service, publicRule)
				if toAdd != nil {
					log.Debug().Interface("toAdd", toAdd).Str("serviceName", service.Name).Msg("Adding new ingress for service")
					di.Ingresses = append(di.Ingresses, IngressesInfo{service.ServiceId, service.ServiceInstanceId, []*v1beta1.Ingress{toAdd}})
				}
			}
		}
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := di.networkDecorator.Build(di)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error building network components")
		return errNetDecorator
	}

	log.Debug().Interface("Ingresses", di.Ingresses).Msg("Ingresses have been build and are ready to deploy")
	return nil
}

func (di *DeployableIngress) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for _, ingresses := range di.Ingresses {
		for _, toCreate := range ingresses.Ingresses {
			log.Debug().Interface("toCreate", toCreate).Msg("Creating ingress")
			created, err := di.client.Create(toCreate)
			if err != nil {
				log.Error().Err(err).Interface("toCreate", toCreate).Msg("cannot create ingress")
				return err
			}
			log.Debug().Str("serviceId", ingresses.ServiceId).Str("uid", string(created.GetUID())).Msg("Ingress has been created")
			numCreated++
			res := entities.NewMonitoredPlatformResource(created.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT], string(created.GetUID()),
				created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
				created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
				created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
			controller.AddMonitoredResource(&res)
		}
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := di.networkDecorator.Deploy(di)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error deploying network components")
		return errNetDecorator
	}

	return nil
}

func (di *DeployableIngress) Undeploy() error {
	deleted := 0
	for _, ingresses := range di.Ingresses {
		for _, toDelete := range ingresses.Ingresses {
			err := di.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
			if err != nil {
				log.Error().Str("serviceId", ingresses.ServiceId).Interface("toDelete", toDelete).Msg("cannot delete ingress")
				return err

			}
			log.Debug().Str("serviceId", ingresses.ServiceId).Str("Name", toDelete.Name).Msg("Ingress has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("Ingresses has been deleted")

	// call the network decorator and modify deployments accordingly
	errNetDecorator := di.networkDecorator.Undeploy(di)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error undeploying network components")
		return errNetDecorator
	}

	return nil

}
