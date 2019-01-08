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
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/rs/zerolog/log"
    "k8s.io/client-go/kubernetes/typed/apps/v1"
    "k8s.io/client-go/kubernetes"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/utils"
    "fmt"
    "github.com/nalej/deployment-manager/pkg/network"
    "strings"
)

const (
    // Name of the Docker ZT agent image
    ZTAgentImageName = "nalejops/zt-agent:v0.1.0"
    // Prefix defining Nalej Services
    NalejServicePrefix = "NALEJ_SERV_"
    // Default imagePullPolicy
    DefaultImagePullPolicy = apiv1.PullAlways
)


// Deployable Deployments
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
    // DNS ips
    dnsHosts []string
    // map of Deployments ready to be deployed
    // service_id -> deployment
    deployments map[string]appsv1.Deployment
    // map of agents deployed for every service
    // service_id -> zt-agent deployment
    ztAgents map[string]appsv1.Deployment
}

func NewDeployableDeployment(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, organizationName string,
    deploymentId string, appInstanceId string, appName string, dnsHosts []string) *DeployableDeployments {
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
        dnsHosts: dnsHosts,
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

    // Create the list of Nalej variables
    nalejVars := d.getNalejEnvVariables()

    for serviceIndex, service := range d.stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d",service.ServiceId,serviceIndex+1,len(d.stage.Services))

        // value for privileged user
        user0 := int64(0)
        privilegedUser := &user0

        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: common.FormatName(service.Name),
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
                        // Set POD DNS policies
                        DNSPolicy: apiv1.DNSNone,
                        DNSConfig: &apiv1.PodDNSConfig{
                            Nameservers: d.dnsHosts,
                        },
                        Containers: []apiv1.Container{
                            // User defined container
                            {
                                Name:  common.FormatName(service.Name),
                                Image: service.Image,
                                Env:   d.getEnvVariables(nalejVars,service.EnvironmentVariables),
                                Ports: d.getContainerPorts(service.ExposedPorts),
                                ImagePullPolicy: DefaultImagePullPolicy,
                            },
                            // ZT sidecar container
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
                                    "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.organizationId,
                                    "--organizationName", d.organizationName,
                                    "--networkId", d.ztNetworkId,
                                },
                                Env: []apiv1.EnvVar{
                                    // Indicate this is not a ZT proxy
                                    {
                                        Name:  "ZT_PROXY",
                                        Value: "false",
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: d.getContainerPorts(service.ExposedPorts),
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

        if service.Credentials != nil {
            log.Debug().Msg("Adding credentials to the deployment")
            deployment.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{
                apiv1.LocalObjectReference{
                    Name: service.ServiceId,
                },
            }
        }

        if service.Configs != nil && len(service.Configs) > 0 {
            log.Debug().Msg("Adding config maps")
            log.Debug().Msg("Creating volumes")
            configVolumes := make([]apiv1.Volume, 0)
            cmVolumeMounts := make([]apiv1.VolumeMount, 0)
            for _, cf := range service.Configs{
                v := &apiv1.Volume{
                    Name:         fmt.Sprintf("cv-%s", cf.ConfigFileId),
                    VolumeSource: apiv1.VolumeSource{
                        ConfigMap:             &apiv1.ConfigMapVolumeSource{
                            LocalObjectReference: apiv1.LocalObjectReference{
                                Name: cf.ConfigFileId,
                            },
                        },
                    },
                }
                configVolumes = append(configVolumes, *v)
                mountPath, _ := GetConfigMapPath(cf.MountPath)
                vm := &apiv1.VolumeMount{
                    Name:             fmt.Sprintf("cv-%s", cf.ConfigFileId),
                    ReadOnly:         true,
                    MountPath:        mountPath,
                }
                cmVolumeMounts = append(cmVolumeMounts, *vm)
            }
            deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, configVolumes...)
            log.Debug().Msg("Linking configmap volumes")
            deployment.Spec.Template.Spec.Containers[0].VolumeMounts = cmVolumeMounts
        }

        // Set a different set of labels to identify this agent
        ztAgentLabels := map[string]string {
            "agent": "zt-agent",
            "app": service.Labels["app"],
        }

        ztAgentName := fmt.Sprintf("zt-%s",common.FormatName(service.Name))
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
                                    "--serviceName", common.FormatName(service.Name),
                                    "--deploymentId", d.deploymentId,
                                    "--fragmentId", d.stage.FragmentId,
                                    "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.organizationId,
                                    "--organizationName", d.organizationName,
                                    "--networkId", d.ztNetworkId,
                                    "--isProxy",
                                },
                                Env: []apiv1.EnvVar{
                                    // Indicate this is a ZT proxy
                                    {
                                        Name:  "ZT_PROXY",
                                        Value: "true",
                                    },
                                    // Indicate the name of the k8s service
                                    {
                                        Name: "K8S_SERVICE_NAME",
                                        Value: common.FormatName(service.Name),
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: d.getContainerPorts(service.ExposedPorts),
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

// Get the set of Nalej environment variables designed to help users.
//  params:
//   nalejVariables set of variables for nalej Services
//  return:
//   list of environment variables
func (d *DeployableDeployments) getNalejEnvVariables() map[string]string {
    // TODO these variables should be generated for the whole fragment
    vars := make(map[string]string,0)
    for _, service := range d.stage.Services {
        name := fmt.Sprintf("%s%s", NalejServicePrefix,strings.ToUpper(service.ServiceId))
        netName := network.GetNetworkingName(service.Name, d.organizationName, d.appInstanceId)
        value := fmt.Sprintf("%s.service.nalej",netName)
        vars[name] = value
    }
    log.Debug().Interface("variables", vars).Msg("Nalej common variables")
    return vars
}


// Transform a service map of environment variables to the corresponding K8s API structure. Any user-defined
// environment variable starting by NALEJ_SERV_ will be replaced if possible.
//  params:
//   variables to be used
//  return:
//   list of k8s environment variables
func (d *DeployableDeployments) getEnvVariables(nalejVariables map[string]string, variables map[string]string) []apiv1.EnvVar {
    result := make([]apiv1.EnvVar, 0)
    for k, v := range variables {
        if strings.HasPrefix(k,NalejServicePrefix) {
            // The key cannot have the NalejServicePrefix, that is a reserved word
            log.Warn().Str("UserEnvironmentVariable", k).Msg("reserved variable name will be ignored")
        } else {
            newValue := v
            // check if we have to replace a NALEJ_SERVICE variable
            for nalejK, nalejVariable := range nalejVariables {
                newValue = strings.Replace(newValue, nalejK, nalejVariable, -1)
            }
            toAdd := apiv1.EnvVar{ Name: k, Value: newValue}
            log.Debug().Str("formerKey", k).Str("formerValue",v).Interface("apiv1EnvVar",toAdd).Msg("environmentVariable")
            result = append(result, toAdd)
        }
    }
    for k, v := range nalejVariables {
        result = append(result, apiv1.EnvVar{Name:k, Value: v})
    }
    log.Debug().Interface("nalej_variables", result).Str("appId", d.appInstanceId).Msg("generated variables for service")
    return result
}


// Transform a Nalej list of exposed ports into a K8s api port.
//  params:
//   ports list of exposed ports
//  return:
//   list of ports into k8s api format
func (d *DeployableDeployments) getContainerPorts(ports []*pbConductor.Port) []apiv1.ContainerPort {
    obtained := make([]apiv1.ContainerPort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name})
    }
    return obtained
}