/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    appsv1 "k8s.io/api/apps/v1"
    apiv1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "fmt"
    "github.com/rs/zerolog/log"
    "k8s.io/apimachinery/pkg/util/intstr"
)

/*
 * Specification of potential k8s deployable resources and their functions.
 */

// Interface for any deployable resource
type DeployableResource interface {
    // Deploy this resource in the given namespace
    Deploy(namespace string) error
    // Undeploy this resource
    Undeploy(namespace string) error
    // Watch this resource
    //Wath() err

}


// the definition of Nalej services defined in a deployment stage into something closer to K8s definition.
type DeployableResources struct {
    // name of the target namespace to use
    targetNamespace string
    // collection of deployments
    deployments []appsv1.Deployment
    // collection of services
    services []apiv1.Service
}


// This function build the data structures required to execute the deployment of a stage into kubernetes.
// The stage definition is analyzed and the whole solution is designed. If the building process fails, the whole
// stage is considered to be wrong and returns an error.
//  params:
//   stage to be processed
//  returns:
//   set of deployable resources to be executed using k8s api, error if something went wrong
func BuildResources(fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage) (*DeployableResources,error) {
    targetNamespace := getNamespace(fragment.AppId)

    deployments,err := buildDeployments(targetNamespace, stage)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to generate deployments for stage %s",stage.StageId)
        return nil, err
    }

    services, err := buildServices(targetNamespace, stage)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to generate services for stage %s",stage.StageId)
        return nil, err
    }

    toReturn := DeployableResources{
        targetNamespace: targetNamespace,
        deployments: deployments,
        services: services,
    }

    return &toReturn, nil
}


// Get the K8s deployment structures for a given deployment fragment.
// params:
//  namespace: target namespace
//  stage definition with the corresponding Nalej services.
// return:
//  corresponding deployments or error if any
func buildDeployments(namespace string, stage *pbConductor.DeploymentStage) ([]appsv1.Deployment, error) {
    // TODO check potential errors.
    log.Debug().Msgf("build deployments for stage %s",stage.StageId)
    deployments:= make([]appsv1.Deployment,0,len(stage.Services))

    for serviceIndex, service := range stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d",service.ServiceId,serviceIndex+1,len(stage.Services))
        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: service.Name,
                Namespace: namespace,
                Labels: service.Labels,
            },
            Spec: appsv1.DeploymentSpec{
                Replicas: int32Ptr(service.Specs.Replicas),
                Selector: &metav1.LabelSelector{
                    MatchLabels: service.Labels,
                },
                Template: apiv1.PodTemplateSpec{
                    ObjectMeta: metav1.ObjectMeta{
                        Labels: service.Labels,
                    },
                    Spec: apiv1.PodSpec{
                        Containers: []apiv1.Container{
                            {
                                Name:  service.Name,
                                Image: service.Image,
                                Env: getEnvVariables(service.EnvironmentVariables),
                                Ports: getContainerPorts(service.ExposedPorts),
                            },
                        },
                    },
                },
            },
        }
        deployments = append(deployments, deployment)
    }
    return deployments, nil
}

// Build the k8s services that expose the ports required by a Nalej service.
//  params:
//   namespace target namespace
//   stage definition with the corresponding Nalej services.
//  return:
//   corresponding services or error if any
func buildServices(namespace string, stage *pbConductor.DeploymentStage)([]apiv1.Service, error) {
    // TODO check potential errors
    log.Debug().Msgf("build services for stage %s",stage.StageId)
    services := make([]apiv1.Service,0,len(stage.Services))
    for serviceIndex, service := range stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(stage.Services))
        k8sService := apiv1.Service{
            Spec: apiv1.ServiceSpec{
                ExternalName: service.Name,
                Ports: getServicePorts(service.ExposedPorts),
            },
        }
        services = append(services, k8sService)
    }
    return services, nil
}

// Transform a Nalej list of exposed ports into a K8s api port.
//  params:
//   ports list of exposed ports
//  return:
//   list of ports into k8s api format
func getContainerPorts(ports []*pbConductor.Port) []apiv1.ContainerPort {
    obtained := make([]apiv1.ContainerPort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name,
            HostPort: p.InternalPort})
    }
    return obtained
}

func getServicePorts(ports []*pbConductor.Port) []apiv1.ServicePort {
    obtained := make([]apiv1.ServicePort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ServicePort{Name: p.Name,
            Port: p.ExposedPort, TargetPort: intstr.IntOrString{IntVal: p.InternalPort}})
    }
    return obtained
}


// Transform a service map of environment variables to the corresponding K8s API structure.
//  params:
//   variables to be used
//  return:
//   list of k8s environment variables
func getEnvVariables(variables map[string]string) []apiv1.EnvVar {
    obtained := make([]apiv1.EnvVar,0,len(variables))
    for _, k := range variables {
        obtained = append(obtained, apiv1.EnvVar{Name: k, Value: variables[k]})
    }
    return obtained
}

// Return the namespace associated with a service.
//  params:
//   appId for the application this namespace is connected to
//  return:
//   associated namespace
func getNamespace(appId *pbConductor.AppDescriptorId) string {
    return fmt.Sprintf("%s-%s", appId.OrganizationId, appId.AppDescriptorId)
}