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
    v12 "k8s.io/client-go/kubernetes/typed/core/v1"

    "fmt"
    "github.com/rs/zerolog/log"
    "k8s.io/apimachinery/pkg/util/intstr"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/typed/apps/v1"

)

/*
 * Specification of potential k8s deployable resources and their functions.
 */

const (
    // Grace period in seconds to delete a deployable.
    DeleteGracePeriod = 10
)



// Definition of a collection of deployable resources contained by a stage. This object is deployable and it has
// deployable objects itself. The deploy of this object simply consists on the deployment of the internal objects.
type DeployableKubernetesStage struct {
    // kubernetes client
    client *kubernetes.Clientset
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // name of the target namespace to use
    namespace *DeployableNamespace
    // collection of deployments
    deployments *DeployableDeployment
    // collection of services
    services *DeployableService
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   client k8s api client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    //targetNamespace string) executor.Deployable {
    targetNamespace string) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client: client,
        stage: stage,
        targetNamespace: targetNamespace,
        namespace: NewDeployableNamespace(client, stage, targetNamespace),
        services: NewDeployableService(client, stage, targetNamespace),
        deployments: NewDeployableDeployment(client, stage, targetNamespace),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build namespace
    err := d.namespace.Build()
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create namespace %s for stageId %s", d.targetNamespace, d.stage.StageId)
        return err
    }

    // Build deployments
    err = d.deployments.Build()
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create deployments for stageId %s", d.stage.StageId)
        return err
    }
    // Build services
    err = d.services.Build()
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create services for stageId %s", d.stage.StageId)
        return err
    }

    return nil
}

func (d DeployableKubernetesStage) Deploy() error {
    // Deploy namespace
    log.Debug().Msgf("build namespace for stage %s",d.stage.StageId)
    err := d.namespace.Deploy()
    if err != nil {
        return err
    }
    // Deploy deployments
    log.Debug().Msgf("build deployments for stage %s",d.stage.StageId)
    err = d.deployments.Deploy()
    if err != nil {
        return err
    }
    // Deploy services
    log.Debug().Msgf("build services for stage %s",d.stage.StageId)
    err = d.services.Deploy()
    if err != nil {
        return err
    }
    return nil
}

func (d DeployableKubernetesStage) Undeploy() error {
    // Deploy namespace
    err := d.namespace.Undeploy()
    if err != nil {
        return err
    }
    // Deploy deployments
    err = d.deployments.Undeploy()
    if err != nil {
        return err
    }
    // Deploy services
    err = d.services.Undeploy()
    if err != nil {
        return err
    }
    return nil
}

// Deployable deployments
//-----------------------

type DeployableDeployment struct{
    // kubernetes client
    client v1.DeploymentInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // deployments
    deployments []appsv1.Deployment
}

func NewDeployableDeployment(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string) *DeployableDeployment {
    return &DeployableDeployment{
        client: client.AppsV1().Deployments(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        deployments: make([]appsv1.Deployment,0),
    }
}

func(d *DeployableDeployment) GetId() string {
    return d.stage.StageId
}

func(d *DeployableDeployment) Build() error {
    // TODO check potential errors.
    deployments:= make([]appsv1.Deployment,0,len(d.stage.Services))

    for serviceIndex, service := range d.stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d",service.ServiceId,serviceIndex+1,len(d.stage.Services))
        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: service.Name,
                Namespace: d.targetNamespace,
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
    d.deployments = deployments
    return nil
}

func(d *DeployableDeployment) Deploy() error {
    for _, dep := range d.deployments {
        _, err := d.client.Create(&dep)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",dep.Name)
            return err
        }
    }
    return nil
}

func(d *DeployableDeployment) Undeploy() error {
    for _, dep := range d.deployments {
        err := d.client.Delete(dep.Name,metav1.NewDeleteOptions(DeleteGracePeriod))
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",dep.Name)
            return err
        }
    }
    return nil
}

// Deployable services
//--------------------

type DeployableService struct {
    // kubernetes client
    client v12.ServiceInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // services
    services []apiv1.Service
}

func NewDeployableService(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string) *DeployableService {

    return &DeployableService{
        client: client.CoreV1().Services(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        services: make([]apiv1.Service,0),
    }
}

func(d *DeployableService) GetId() string {
    return d.stage.StageId
}

func(s *DeployableService) Build() error {
    // TODO check potential errors
    services := make([]apiv1.Service,0,len(s.stage.Services))
    for serviceIndex, service := range s.stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.stage.Services))
        k8sService := apiv1.Service{
            Spec: apiv1.ServiceSpec{
                ExternalName: service.Name,
                Ports: getServicePorts(service.ExposedPorts),
            },
        }
        services = append(services, k8sService)
    }
    return nil
}

func(s *DeployableService) Deploy() error {
    for _, serv := range s.services {
        created, err := s.client.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",serv.Name)
            return err
        }
        s.services = append(s.services, *created)
    }
    return nil
}

func(s *DeployableService) Undeploy() error {
    for _, serv := range s.services {
        err := s.client.Delete(serv.Name, metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",serv.Name)
            return err
        }
    }
    return nil
}


// Deployable namespace
//--------------------

type DeployableNamespace struct {
    // kubernetes client
    client v12.NamespaceInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // namespace
    namespace apiv1.Namespace
}

func NewDeployableNamespace(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string) *DeployableNamespace {
    return &DeployableNamespace{
        client:          client.CoreV1().Namespaces(),
        stage:           stage,
        targetNamespace: targetNamespace,
        namespace:       apiv1.Namespace{},
    }
}

func(n *DeployableNamespace) GetId() string {
    return n.stage.StageId
}

func(n *DeployableNamespace) Build() error {
    ns := apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n.targetNamespace}}
    n.namespace = ns
    return nil
}

func(n *DeployableNamespace) Deploy() error {
    retrieved, err := n.client.Get(n.targetNamespace,metav1.GetOptions{IncludeUninitialized: true})

    if retrieved.Name!="" {
        n.namespace = *retrieved
        log.Warn().Msgf("namespace %s already exists",n.targetNamespace)
        return nil
    }
    created, err := n.client.Create(&n.namespace)
    n.namespace = *created
    return err
}

func(n *DeployableNamespace) Undeploy() error {
    log.Debug().Msgf("the namespace object is %v\n",n)
    err := n.client.Delete(n.targetNamespace, metav1.NewDeleteOptions(DeleteGracePeriod))
    return err
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
    //return fmt.Sprintf("%s", appId.AppDescriptorId)
    //return fmt.Sprintf("myspace")

}