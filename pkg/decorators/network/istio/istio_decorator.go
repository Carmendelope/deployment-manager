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
    "github.com/nalej/deployment-manager/pkg/config"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/deployment-manager/pkg/utils"
    "github.com/nalej/derrors"
    "github.com/nalej/grpc-application-go"
    "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    "istio.io/api/networking/v1alpha3"
    networking "istio.io/client-go/pkg/apis/networking/v1alpha3"
    versionedclient "istio.io/client-go/pkg/clientset/versioned"
    metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/rest"
)

const(
    IstioLabelInjection = "istio-injection"
    InstPrefixLength = 6
    OrgPrefixLength = 8
)

type IstioDecorator struct {
    // Istio client
    Client *versionedclient.Clientset
}

func NewIstioDecorator() (executor.NetworkDecorator, derrors.Error){

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

    return &IstioDecorator{Client: ic}, nil

}


func (id *IstioDecorator) Build(aux executor.Deployable, args ...interface{}) derrors.Error {

    switch target := aux.(type) {
    // Process a namespace
    case *kubernetes.DeployableNamespace:
        return id.decorateNamespace(target)
    case *kubernetes.DeployableDeployments:
        return id.decorateDeployments(target)
    case *kubernetes.DeployableServices:
        return id.decorateServices(target)
    default:
        // nothing to do
        return nil
    }
    return nil
}

func (id *IstioDecorator) Deploy(aux executor.Deployable, args ...interface{}) derrors.Error {
   // Ingresses use the the Istio gateway solution
   /*
   switch target := aux.(type){
   case *kubernetes.DeployableIngress:
       return id.deployIstioIngress(target)
   }
   */

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
    // Those services connected with the ingress we have to disable the inbound ports
    /*
    for _, publicRule := range target.Data.Stage.PublicRules {
        found := false
        for _, s := range target.Services{
            // If we already have a service for this public rule skip to the next one
            if publicRule.TargetServiceGroupInstanceId == s.Service.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] &&
                publicRule.TargetServiceInstanceId == s.Service.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] {
                found = true
                break
            }
        }
        if !found {
            // Create a service
            newServ := apiv1.Service{
                ObjectMeta: metaV1.ObjectMeta{
                    Name:
                },
                Spec: {},
            }
        }
    }
    */

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
                log.Debug().Str("serviceName",dep.Name).Msg("candidate for Istio ingress endpoint")
                // Set the corresponding flags
                if dep.Spec.Template.Annotations == nil {
                    dep.Spec.Template.Annotations = make(map[string]string,0)
                }
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

// Deploy an ingress for a service to be exposed. This is translated into a gateway entry and
// its corresponding virtual service.
func (id *IstioDecorator) deployIstioIngress(ingress *kubernetes.DeployableIngress) derrors.Error {

    // we will generate one gateway per service
    gateways := make(map[string]*networking.Gateway,0)
    // we have a map of pending virtual services
    virtualServices := make(map[string]*networking.VirtualService)

    // Iterate through services and create ingresses when they require them
    for _, publicRule := range ingress.Data.Stage.PublicRules {
        log.Debug().Interface("rule", publicRule).Msg("Checking public rule")
        for _, service := range ingress.Data.Stage.Services {
            log.Debug().Interface("service", service).Msg("Checking service for istio public ingress")
            if publicRule.TargetServiceGroupInstanceId == service.ServiceGroupInstanceId && publicRule.TargetServiceInstanceId == service.ServiceInstanceId {
                // create a gateway if not done yet and add as many ports as required
                gateway, found := gateways[service.Name]
                if !found {
                    gateway = &networking.Gateway{}
                    gateways[service.Name] = gateway
                }
                // update the gateway with new information about ports following this rule
                id.updateIstioGateway(gateway, ingress, service, publicRule)
                gateways[service.Name] = gateway

                // Every service requires an Istio virtual service connected with a gateway
                vs := id.createIstioVirtualService(gateway, ingress, service, publicRule)
                virtualServices[service.Name] = vs
            }
        }
    }


    // Deploy all the gateways
    for name, gw := range gateways {
        log.Debug().Str("namespace", gw.Namespace).Str("gateway", name).Msg("create gateway")
        _, err := id.Client.NetworkingV1alpha3().Gateways(ingress.Data.Namespace).Create(gw)
        if err != nil {
            log.Error().Interface("gateway", gw).Err(err).Msg("error generating gateway")
            return derrors.NewInternalError("impossible to generate gateway", err)
        }
    }
    // Deploy all the virtual services
    for name, vs := range virtualServices {
        log.Debug().Str("namespace", vs.Namespace).Str("virtualservice", name).Msg("create virtualservice")
        _, err := id.Client.NetworkingV1alpha3().VirtualServices(ingress.Data.Namespace).Create(vs)
        if err != nil {
            log.Error().Interface("virtualservice", vs).Err(err).Msg("error generating virtualservice")
            return derrors.NewInternalError("impossible to generate virtual service", err)
        }
    }

    return nil
}


func(id *IstioDecorator) updateIstioGateway(gateway *networking.Gateway, ingress *kubernetes.DeployableIngress,
    service *grpc_application_go.ServiceInstance,
    publicRule *grpc_conductor_go.PublicSecurityRuleInstance){

    // create a gateway for the service

    // get the corresponding names for this ingress
    ingressName, serviceGroupInstPrefix, appInstPrefix, _ := id.getNamePrefixes(service, publicRule)

    // the gateway was not initialized
    if gateway.Spec.Servers == nil{

        gateway.ObjectMeta = metaV1.ObjectMeta{
            Name: service.Name,
            Namespace: ingress.Data.Namespace,
            Labels: id.generateLabels(ingress, service),
        }

        // Use default Istio gateway
        gateway.Spec.Selector = map[string]string{"istio": "ingressgateway"}
        // Initialize servers entry
        gateway.Spec.Servers = make([]*v1alpha3.Server,0)
    }

    // Add a server following the public rule
    newServer := v1alpha3.Server{
        Hosts: []string{
            fmt.Sprintf("%s.%s.%s.appcluster.%s", ingressName, serviceGroupInstPrefix, appInstPrefix, config.GetConfig().ClusterPublicHostname),
        },
        Port: &v1alpha3.Port{
            Number: uint32(publicRule.TargetPort),
            Protocol: "http",
            Name: fmt.Sprintf("port-%d-http",publicRule.TargetPort),
        },
    }

    gateway.Spec.Servers = append(gateway.Spec.Servers, &newServer)

}

func(id *IstioDecorator) createIstioVirtualService(gateway *networking.Gateway, ingress *kubernetes.DeployableIngress,
    service *grpc_application_go.ServiceInstance,
    publicRule *grpc_conductor_go.PublicSecurityRuleInstance) *networking.VirtualService {

    // create a virtual service to expose the ingress
    vs := &networking.VirtualService{
        ObjectMeta: metaV1.ObjectMeta{
            Name: service.Name,
            Labels: id.generateLabels(ingress, service),
        },
        Spec: v1alpha3.VirtualService{
            Gateways:[]string{gateway.Name},
            Hosts: []string{"*"},
            Http:[]*v1alpha3.HTTPRoute{
                {
                    Route: []*v1alpha3.HTTPRouteDestination{
                        {
                            Destination: &v1alpha3.Destination{
                                Port: &v1alpha3.PortSelector{
                                    Number: uint32(publicRule.TargetPort),
                                },
                                Host: service.Name,
                            },
                        },
                    },
                },
                },
            },
        }


    return vs
}


func(id *IstioDecorator) generateLabels(d *kubernetes.DeployableIngress,
    service *grpc_application_go.ServiceInstance) map[string]string {

    extendedLabels := make(map[string]string, 0)
    if service.Labels != nil {
        // users have already defined labels for this app
        for k,v := range service.Labels {
            extendedLabels[k] = v
        }
    }

    extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.Data.FragmentId
    extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = d.Data.OrganizationId
    extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.Data.AppDescriptorId
    extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.Data.AppInstanceId
    extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = d.Data.Stage.StageId
    extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_ID] = d.Data.DeploymentId
    extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.Data.Stage.FragmentId
    extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
    extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
    extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
    extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
    extendedLabels["clusters"] = "application"
    return extendedLabels
}

func(id *IstioDecorator) getNamePrefixes(service *grpc_application_go.ServiceInstance,
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
