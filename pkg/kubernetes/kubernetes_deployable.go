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

    "github.com/rs/zerolog/log"
    "k8s.io/apimachinery/pkg/util/intstr"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/typed/apps/v1"

    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/monitor"
    "github.com/nalej/deployment-manager/pkg/utils"
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
    // kubernetes Client
    client *kubernetes.Clientset
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // ZeroTier network id
    ztNetworkId string
    // collection of deployments
    deployments *DeployableDeployments
    // collection of services
    services *DeployableServices
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, deploymentId string,
    appInstanceId string) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client: client,
        stage: stage,
        targetNamespace: targetNamespace,
        ztNetworkId: ztNetworkId,
        services: NewDeployableService(client, stage, targetNamespace),
        deployments: NewDeployableDeployment(client, stage, targetNamespace,ztNetworkId, organizationId, deploymentId,
            appInstanceId),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build deployments
    err := d.deployments.Build()
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

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

    // Deploy deployments
    log.Debug().Msgf("build deployments for stage %s",d.stage.StageId)
    err := d.deployments.Deploy(controller)
    if err != nil {
        return err
    }
    // Deploy services
    log.Debug().Msgf("build services for stage %s",d.stage.StageId)
    err = d.services.Deploy(controller)
    if err != nil {
        return err
    }
    return nil
}

func (d DeployableKubernetesStage) Undeploy() error {
    // Deploying the namespace should be enough
    // Deploy namespace
    /*
    err := d.namespace.Undeploy()
    if err != nil {
        return err
    }
    */
    // Deploy deployments
    err := d.deployments.Undeploy()
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

type DeployableDeployments struct{
    // kubernetes Client
    client v1.DeploymentInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // zero-tier network id
    ztNetworkId string
    // organization id
    organizationId string
    // deployment id
    deploymentId string
    // application instance id
    appInstanceId string
    // map of deployments ready to be deployed
    // service_id -> deployment
    deployments map[string]appsv1.Deployment
}

func NewDeployableDeployment(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, deploymentId string,
    appInstanceId string) *DeployableDeployments {
    return &DeployableDeployments{
        client: client.AppsV1().Deployments(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        ztNetworkId: ztNetworkId,
        organizationId: organizationId,
        deploymentId: deploymentId,
        appInstanceId: appInstanceId,
        deployments: make(map[string]appsv1.Deployment,0),
    }
}

func(d *DeployableDeployments) GetId() string {
    return d.stage.StageId
}

func(d *DeployableDeployments) Build() error {
    // TODO check potential errors.
    deployments:= make(map[string]appsv1.Deployment,0)

    for serviceIndex, service := range d.stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d",service.ServiceId,serviceIndex+1,len(d.stage.Services))
        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: service.Name,
                Namespace: d.targetNamespace,
                Labels: service.Labels,
                Annotations: map[string] string {
                    utils.NALEJ_SERVICE_NAME : service.ServiceId,
                },
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
                    // Every pod template is designed to use a container with the requested image
                    // and a helping sidecar with a containerized zerotier that joins the network
                    // after running
                    Spec: apiv1.PodSpec{
                        Containers: []apiv1.Container{
                            {
                                Name:  service.Name,
                                Image: service.Image,
                                Env: getEnvVariables(service.EnvironmentVariables),
                                Ports: getContainerPorts(service.ExposedPorts),
                            },
                            // zero-tier sidecar
                            {
                                Name: "zt-agent",
                                // TODO prepare this to be pulled from a public docker repository
                                Image: "nalej/zt-agent:v0.1.0",
                                Args: []string{
                                    "run",
                                    "--appInstanceId", d.appInstanceId,
                                    "--deploymentId", d.deploymentId,
                                    "--fragmentId", d.stage.FragmentId,
                                    "--hostname", "$(HOSTNAME)",
                                    "--managerAddr", "10.0.2.2:5200",
                                    //"--managerAddr", "10.0.2.2:$($DEPLOYMENT_MANAGER_SERVICE_PORT)",
                                    //"--managerAddr", fmt.Sprintf("deployment-manager.nalej:$($DEPLOYMENT_MANAGER_SERVICE_PORT)"),
                                    "--organizationId", d.organizationId,
                                    "--networkId", d.ztNetworkId,
                                },
                                Env: []apiv1.EnvVar{
                                    apiv1.EnvVar{Name:"HOSTNAME", ValueFrom: &apiv1.EnvVarSource{FieldRef: &apiv1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
                                },
                                SecurityContext:
                                    &apiv1.SecurityContext{
                                        Privileged: boolPtr(true),
                                        Capabilities: &apiv1.Capabilities{
                                            Add: [] apiv1.Capability{
                                              "NET_ADMIN", "SYS_ADMIN",
                                            },
                                        },
                                    },
                                VolumeMounts: []apiv1.VolumeMount{
                                    {
                                        Name: "dev-net-tun",
                                        ReadOnly: true,
                                        MountPath: "/dev/net/tun",
                                    },
                                },
                            },
                        },
                        Volumes: []apiv1.Volume{
                            // zerotier sidecar volume
                            {
                                Name: "dev-net-tun",
                                VolumeSource: apiv1.VolumeSource{
                                    HostPath: &apiv1.HostPathVolumeSource{
                                        Path: "/dev/net/tun",
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }

        deployments[service.ServiceId] = deployment
    }
    d.deployments = deployments
    return nil
}

func(d *DeployableDeployments) Deploy(controller executor.DeploymentController) error {
    for serviceId, dep := range d.deployments {
        deployed, err := d.client.Create(&dep)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",dep.Name)
            return err
        }
        log.Debug().Msgf("created deployment with uid %s", deployed.GetUID())
        controller.AddMonitoredResource(string(deployed.GetUID()), serviceId, d.stage.StageId)
    }
    return nil
}

func(d *DeployableDeployments) Undeploy() error {
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

type DeployableServices struct {
    // kubernetes Client
    client v12.ServiceInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // serviceId -> serviceInstance
    services map[string]apiv1.Service
}

func NewDeployableService(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string) *DeployableServices {

    return &DeployableServices{
        client: client.CoreV1().Services(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        services: make(map[string]apiv1.Service,0),
    }
}

func(d *DeployableServices) GetId() string {
    return d.stage.StageId
}

func(s *DeployableServices) Build() error {
    // TODO check potential errors
    services := make(map[string]apiv1.Service,0)
    for serviceIndex, service := range s.stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.stage.Services))
        ports := getServicePorts(service.ExposedPorts)
        if ports!=nil{
            k8sService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.targetNamespace,
                    Name: service.Name,
                    Labels: service.Labels,
                    Annotations: map[string] string {
                        "nalej-service" : service.ServiceId,
                    },
                },
                Spec: apiv1.ServiceSpec{
                    ExternalName: service.Name,
                    Ports: getServicePorts(service.ExposedPorts),
                    // TODO remove by default we use clusterip.
                    Type: apiv1.ServiceTypeNodePort,
                    Selector: service.Labels,
                },

            }
            services[service.ServiceId] = k8sService
        } else {
            log.Debug().Msgf("No k8s service is generated for %s",service.ServiceId)
        }
    }
    // add the created services
    s.services = services
    return nil
}

func(s *DeployableServices) Deploy(controller executor.DeploymentController) error {
    for serviceId, serv := range s.services {
        created, err := s.client.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",serv.Name)
            return err
        }
        log.Debug().Msgf("created service with uid %s", created.GetUID())
        controller.AddMonitoredResource(string(created.GetUID()), serviceId,s.stage.StageId)
    }
    return nil
}

func(s *DeployableServices) Undeploy() error {
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
// A namespace is associated with a fragment. This is a special case of deployable that is only intended to be
// executed before the fragment deployment starts.
//--------------------

type DeployableNamespace struct {
    // kubernetes Client
    client v12.NamespaceInterface
    // fragment this namespace is attached to
    fragmentId string
    // namespace name descriptor
    targetNamespace string
    // namespace
    namespace apiv1.Namespace
}

func NewDeployableNamespace(client *kubernetes.Clientset, fragmentId string, targetNamespace string) *DeployableNamespace {
    return &DeployableNamespace{
        client:          client.CoreV1().Namespaces(),
        fragmentId:      fragmentId,
        targetNamespace: targetNamespace,
        namespace:       apiv1.Namespace{},
    }
}

func(n *DeployableNamespace) GetId() string {
    return n.fragmentId
}

func(n *DeployableNamespace) Build() error {
    ns := apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n.targetNamespace}}
    n.namespace = ns
    return nil
}

func(n *DeployableNamespace) Deploy(controller executor.DeploymentController) error {
    retrieved, err := n.client.Get(n.targetNamespace,metav1.GetOptions{IncludeUninitialized: true})

    if retrieved.Name!="" {
        n.namespace = *retrieved
        log.Warn().Msgf("namespace %s already exists",n.targetNamespace)
        return nil
    }
    created, err := n.client.Create(&n.namespace)
    if err != nil {
        return err
    }
    log.Debug().Msgf("invoked namespace with uid %s", string(created.Namespace))
    n.namespace = *created
    // The namespace is a special case that covers all the services
    controller.AddMonitoredResource(string(created.GetUID()), monitor.AllServices,n.fragmentId)
    return err
}

func(n *DeployableNamespace) Undeploy() error {
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
    if len(obtained) == 0 {
        return nil
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
    for k, v := range variables {
        obtained = append(obtained, apiv1.EnvVar{Name: k, Value: v})
    }
    return obtained
}

