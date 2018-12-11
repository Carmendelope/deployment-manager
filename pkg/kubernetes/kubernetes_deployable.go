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
    "github.com/nalej/deployment-manager/pkg"
    "fmt"
)

/*
 * Specification of potential k8s deployable resources and their functions.
 */

const (
    // Grace period in seconds to delete a deployable.
    DeleteGracePeriod = 10
    // Name of the Docker ZT agent image
    // ZTAgentImageName = "nalejregistry.azurecr.io/nalej/zt-agent:v0.1.0"
    ZTAgentImageName = "nalejops/zt-agent:v0.1.0"
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
    // Collection of ingresses to be deployed
    ingresses *DeployableIngress
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, organizationName string,
    deploymentId string, appInstanceId string, appName string, clusterPublicHostname string) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client: client,
        stage: stage,
        targetNamespace: targetNamespace,
        ztNetworkId: ztNetworkId,
        services: NewDeployableService(client, stage, targetNamespace),
        deployments: NewDeployableDeployment(client, stage, targetNamespace,ztNetworkId, organizationId,
            organizationName, deploymentId, appInstanceId, appName),
        ingresses: NewDeployableIngress(client, stage, targetNamespace, clusterPublicHostname),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build deployments
    err := d.deployments.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create deployments")
        return err
    }
    // Build services
    err = d.services.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create services for")
        return err
    }

    err = d.ingresses.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create ingresses")
        return err
    }

    return nil
}

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

    // Deploy deployments
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy deployments")
    err := d.deployments.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying deployments, aborting")
        return err
    }
    // Deploy services
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy services")
    err = d.services.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying services, aborting")
        return err
    }

    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy ingresses")
    err = d.ingresses.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying ingresses, aborting")
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
    err = d.ingresses.Undeploy()
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
    // organization name
    organizationName string
    // deployment id
    deploymentId string
    // application instance id
    appInstanceId string
    // app nalej name
    appName string
    // map of deployments ready to be deployed
    // service_id -> deployment
    deployments map[string]appsv1.Deployment
    // map of agents deployed for every service
    // service_id -> zt-agent deployment
    ztAgents map[string]appsv1.Deployment
}

func NewDeployableDeployment(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, organizationName string,
    deploymentId string, appInstanceId string, appName string) *DeployableDeployments {
    return &DeployableDeployments{
        client: client.AppsV1().Deployments(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        ztNetworkId: ztNetworkId,
        organizationId: organizationId,
        organizationName: organizationName,
        deploymentId: deploymentId,
        appInstanceId: appInstanceId,
        appName: appName,
        deployments: make(map[string]appsv1.Deployment,0),
        ztAgents: make(map[string]appsv1.Deployment,0),
    }
}

func(d *DeployableDeployments) GetId() string {
    return d.stage.StageId
}

func(d *DeployableDeployments) Build() error {

    deployments:= make(map[string]appsv1.Deployment,0)
    agents:= make(map[string]appsv1.Deployment,0)

    for serviceIndex, service := range d.stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d",service.ServiceId,serviceIndex+1,len(d.stage.Services))

        // value for privileged user
        user0 := int64(0)
        privilegedUser := &user0

        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: pkg.FormatName(service.Name),
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
                    Spec: apiv1.PodSpec{
                        Containers: []apiv1.Container{
                            {
                                Name:  pkg.FormatName(service.Name),
                                Image: service.Image,
                                Env:   getEnvVariables(service.EnvironmentVariables),
                                Ports: getContainerPorts(service.ExposedPorts),
                            },
                            {
                                Name: "zt-sidecar",
                                Image: ZTAgentImageName,
                                Args: []string{
                                    "run",
                                    "--appInstanceId", d.appInstanceId,
                                    "--appName", d.appName,
                                    "--serviceName", service.Name,
                                    "--deploymentId", d.deploymentId,
                                    "--fragmentId", d.stage.FragmentId,
                                    "--managerAddr", pkg.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.organizationId,
                                    "--organizationName", d.organizationName,
                                    "--networkId", d.ztNetworkId,
                                },
                                Env: []apiv1.EnvVar{
                                    // Indicate this is not a ZT proxy
                                    apiv1.EnvVar{
                                        Name:  "ZT_PROXY",
                                        Value: "false",
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: getContainerPorts(service.ExposedPorts),
                                SecurityContext:
                                &apiv1.SecurityContext{
                                    RunAsUser: privilegedUser,
                                    Privileged: boolPtr(true),
                                    Capabilities: &apiv1.Capabilities{
                                        Add: [] apiv1.Capability{
                                            "NET_ADMIN",
                                            "SYS_ADMIN",
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
        // Set a different set of labels to identify this agent
        ztAgentLabels := map[string]string {
            "agent": "zt-agent",
            "app": service.Labels["app"],
        }

        ztAgentName := fmt.Sprintf("zt-%s",pkg.FormatName(service.Name))
        agent := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: ztAgentName,
                Namespace: d.targetNamespace,
                Labels: ztAgentLabels,
                Annotations: map[string] string {
                    utils.NALEJ_SERVICE_NAME : service.ServiceId,
                },
            },
            Spec: appsv1.DeploymentSpec{
                Replicas: int32Ptr(1),
                Selector: &metav1.LabelSelector{
                    MatchLabels:ztAgentLabels,
                },
                Template: apiv1.PodTemplateSpec{
                    ObjectMeta: metav1.ObjectMeta{
                        Labels: ztAgentLabels,
                    },
                    // Every pod template is designed to use a container with the requested image
                    // and a helping sidecar with a containerized zerotier that joins the network
                    // after running
                    Spec: apiv1.PodSpec{
                        Containers: []apiv1.Container{
                            // zero-tier sidecar
                            {
                                Name: ztAgentName,
                                Image: ZTAgentImageName,
                                Args: []string{
                                    "run",
                                    "--appInstanceId", d.appInstanceId,
                                    "--appName", d.appName,
                                    "--serviceName", pkg.FormatName(service.Name),
                                    "--deploymentId", d.deploymentId,
                                    "--fragmentId", d.stage.FragmentId,
                                    "--managerAddr", pkg.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.organizationId,
                                    "--organizationName", d.organizationName,
                                    "--networkId", d.ztNetworkId,
                                    "--isProxy",
                                },
                                Env: []apiv1.EnvVar{
                                    // Indicate this is a ZT proxy
                                    apiv1.EnvVar{
                                        Name:  "ZT_PROXY",
                                        Value: "true",
                                    },
                                    // Indicate the name of the k8s service
                                    apiv1.EnvVar{
                                        Name: "K8S_SERVICE_NAME",
                                        Value: pkg.FormatName(service.Name),
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: getContainerPorts(service.ExposedPorts),
                                SecurityContext:
                                    &apiv1.SecurityContext{
                                        RunAsUser: privilegedUser,
                                        Privileged: boolPtr(true),
                                        Capabilities: &apiv1.Capabilities{
                                            Add: [] apiv1.Capability{
                                                "NET_ADMIN",
                                                "SYS_ADMIN",
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
        agents[service.ServiceId] = agent
    }


    d.deployments = deployments
    d.ztAgents = agents
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
    // same approach for agents
    for serviceId, dep := range d.ztAgents {
        deployed, err := d.client.Create(&dep)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment for zt-agent %s",dep.Name)
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
    // service_id -> zt-agent service
    ztAgents map[string]apiv1.Service
}

func NewDeployableService(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string) *DeployableServices {

    return &DeployableServices{
        client: client.CoreV1().Services(targetNamespace),
        stage: stage,
        targetNamespace: targetNamespace,
        services: make(map[string]apiv1.Service,0),
        ztAgents: make(map[string]apiv1.Service,0),
    }
}

func(d *DeployableServices) GetId() string {
    return d.stage.StageId
}

func(s *DeployableServices) Build() error {
    // TODO check potential errors
    services := make(map[string]apiv1.Service,0)
    ztServices := make(map[string]apiv1.Service,0)
    for serviceIndex, service := range s.stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.stage.Services))
        ports := getServicePorts(service.ExposedPorts)
        if ports!=nil{
            k8sService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.targetNamespace,
                    Name: pkg.FormatName(service.Name),
                    Labels: service.Labels,
                    Annotations: map[string] string {
                        "nalej-service" : service.ServiceId,
                    },
                },
                Spec: apiv1.ServiceSpec{
                    ExternalName: pkg.FormatName(service.Name),
                    Ports: ports,
                    // TODO remove by default we use clusterip.
                    Type: apiv1.ServiceTypeNodePort,
                    Selector: service.Labels,
                },
            }
            services[service.ServiceId] = k8sService

            // Create the zt-agent service
            // Set a different set of labels to identify this agent
            ztAgentLabels := map[string]string {
                "agent": "zt-agent",
                "app": service.Labels["app"],
            }

            ztServiceName := fmt.Sprintf("zt-%s",pkg.FormatName(service.Name))
            ztService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.targetNamespace,
                    Name: ztServiceName,
                    Labels: ztAgentLabels,
                    Annotations: map[string] string {
                        "nalej-service" : service.ServiceId,
                    },
                },
                Spec: apiv1.ServiceSpec{
                    ExternalName: ztServiceName,
                    Ports: getServicePorts(service.ExposedPorts),
                    // TODO remove by default we use clusterip.
                    Type: apiv1.ServiceTypeNodePort,
                    Selector: ztAgentLabels,
                },
            }
            ztServices[service.ServiceId] = ztService

            log.Debug().Interface("deployment", k8sService).Msg("generated deployment")

        } else {
            log.Debug().Msgf("No k8s service is generated for %s",service.ServiceId)
        }
    }

    // add the created services
    s.services = services
    s.ztAgents = ztServices
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

    // Create services for agents
    for serviceId, serv := range s.ztAgents {
        created, err := s.client.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service agent %s",serv.Name)
            return err
        }
        log.Debug().Msgf("created service agent with uid %s", created.GetUID())
        controller.AddMonitoredResource(string(created.GetUID()), serviceId,s.stage.StageId)
    }
    return nil
}

func(s *DeployableServices) Undeploy() error {
    for _, serv := range s.services {
        err := s.client.Delete(pkg.FormatName(serv.Name), metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service %s",serv.Name)
            return err
        }
    }
    // undeploy zt agents
    for _, serv := range s.ztAgents {
        err := s.client.Delete(serv.Name, metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service agent %s",serv.Name)
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
    retrieved, err := n.client.Get(n.targetNamespace, metav1.GetOptions{IncludeUninitialized: true})

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

func (n *DeployableNamespace) exists() bool{
    _, err := n.client.Get(n.targetNamespace, metav1.GetOptions{IncludeUninitialized: true})
    return err == nil
}

func(n *DeployableNamespace) Undeploy() error {
    if !n.exists(){
        log.Warn().Str("targetNamespace", n.targetNamespace).Msg("Target namespace does not exists, considering undeploy successful")
        return nil
    }
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
        obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name})
    }
    return obtained
}

func getServicePorts(ports []*pbConductor.Port) []apiv1.ServicePort {
    obtained := make([]apiv1.ServicePort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ServicePort{
            Name: p.Name,
            Port: p.ExposedPort,
            TargetPort: intstr.IntOrString{IntVal: p.InternalPort},
        })
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

