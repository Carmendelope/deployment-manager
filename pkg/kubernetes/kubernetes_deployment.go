/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    "fmt"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/utils"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    appsv1 "k8s.io/api/apps/v1"
    apiv1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/typed/apps/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "strings"
    "github.com/nalej/grpc-application-go"
)

const (
    // Name of the Docker ZT agent image
    ZTAgentImageName = "nalejops/zt-agent:v0.2.0"
    // Prefix defining Nalej Services
    NalejServicePrefix = "NALEJ_SERV_"
    // Default imagePullPolicy
    DefaultImagePullPolicy = apiv1.PullAlways
    //Default storage size
    DefaultStorageAllocationSize = int64(100*1024*1024)
)


// Deployable Deployments
//-----------------------

// Struct for internal use containing information for a deployment. This is intended to be used by the monitoring service.
type DeploymentInfo struct {
    ServiceId string
    ServiceInstanceId string
    Deployment appsv1.Deployment
}

type DeployableDeployments struct{
    // kubernetes Client
    client v1.DeploymentInterface
    // stage metadata
    data entities.DeploymentMetadata
    // array of Deployments ready to be deployed
    // [[service_id, service_instance_id, deployment],...]
    deployments []DeploymentInfo
    // array of agents deployed for every service
    // [[service_id, service_instance_id, deployment],...]
    ztAgents []DeploymentInfo
}

func NewDeployableDeployment(
    client *kubernetes.Clientset, data entities.DeploymentMetadata) *DeployableDeployments {
    return &DeployableDeployments{
        client: client.AppsV1().Deployments(data.Namespace),
        data: data,
        deployments: make([]DeploymentInfo,0),
        ztAgents: make([]DeploymentInfo,0),
    }
}

func(d *DeployableDeployments) GetId() string {
    return d.data.Stage.StageId
}

func(d *DeployableDeployments) Build() error {

    //deployments:= make(map[string]appsv1.Deployment,0)
    //agents:= make(map[string]appsv1.Deployment,0)

    for serviceIndex, service := range d.data.Stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d", service.ServiceId,serviceIndex+1,len(d.data.Stage.Services))

        // value for privileged user
        user0 := int64(0)
        privilegedUser := &user0

        extendedLabels := service.Labels
        extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION] = d.data.OrganizationId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.data.AppDescriptorId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.data.AppInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = d.data.Stage.StageId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = d.data.ServiceGroupId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = d.data.ServiceGroupInstanceId


        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: common.FormatName(service.Name),
                Namespace: d.data.Namespace,
                Labels: extendedLabels,
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
                            Nameservers: d.data.DNSHosts,
                        },
                        Containers: []apiv1.Container{
                            // User defined container
                            {
                                Name:  common.FormatName(service.Name),
                                Image: service.Image,
                                Env:   d.getEnvVariables(d.data.NalejVariables,service.EnvironmentVariables),
                                Ports: d.getContainerPorts(service.ExposedPorts),
                                ImagePullPolicy: DefaultImagePullPolicy,
                            },
                            // ZT sidecar container
                            {
                                Name: "zt-sidecar",
                                Image: ZTAgentImageName,
                                Args: []string{
                                    "run",
                                    "--appInstanceId", d.data.AppInstanceId,
                                    "--appName", d.data.AppName,
                                    "--serviceName", service.Name,
                                    "--deploymentId", d.data.DeploymentId,
                                    "--fragmentId", d.data.Stage.FragmentId,
                                    "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.data.OrganizationId,
                                    "--organizationName", d.data.OrganizationName,
                                    "--networkId", d.data.ZtNetworkId,
                                },
                                Env: []apiv1.EnvVar{
                                    // Indicate this is not a ZT proxy
                                    {
                                        Name:  "ZT_PROXY",
                                        Value: "false",
                                    },
                                },
                                LivenessProbe: &apiv1.Probe{
                                    InitialDelaySeconds: 20,
                                    PeriodSeconds:       60,
                                    TimeoutSeconds:      20,
                                    Handler: apiv1.Handler{
                                        Exec: &apiv1.ExecAction{
                                            Command: []string{
                                                "./nalej/zt-agent",
                                                "check",
                                                "--appInstanceId", d.data.AppInstanceId,
                                                "--appName", d.data.AppName,
                                                "--serviceName", service.Name,
                                                "--deploymentId", d.data.DeploymentId,
                                                "--fragmentId", d.data.Stage.FragmentId,
                                                "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                                "--organizationId", d.data.OrganizationId,
                                                "--organizationName", d.data.OrganizationName,
                                                "--networkId", d.data.ZtNetworkId,
                                            },
                                        },
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: d.getContainerPorts(service.ExposedPorts),
                                ImagePullPolicy: DefaultImagePullPolicy,
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

        if service.RunArguments != nil {
            log.Debug().Msg("Adding arguments")
            deployment.Spec.Template.Spec.Containers[0].Args = service.RunArguments
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
        if service.Storage != nil && len(service.Storage) > 0 {
            // Set VolumeMounts and Volumes based on storage type
            volumes := make([]apiv1.Volume,0)
            volumeMounts := make([]apiv1.VolumeMount,0)

            for i,storage := range service.Storage {
                if storage.Size == 0 {
                    storage.Size = DefaultStorageAllocationSize
                }
                var v *apiv1.Volume
                if storage.Type == grpc_application_go.StorageType_EPHEMERAL {
                    v = &apiv1.Volume {
                        Name: fmt.Sprintf("vol-1%d",i),
                        VolumeSource: apiv1.VolumeSource{
                            EmptyDir:&apiv1.EmptyDirVolumeSource{
                                Medium: apiv1.StorageMediumDefault,
                                SizeLimit:resource.NewQuantity(storage.Size,resource.BinarySI),
                            },
                        },
                    }
                } else {
                    v = &apiv1.Volume {
                        Name: fmt.Sprintf("vol-1%d",i),
                        VolumeSource: apiv1.VolumeSource{
                            PersistentVolumeClaim:&apiv1.PersistentVolumeClaimVolumeSource{
                                // claim name should be same as pvcID that was generated in BuildStorageForServices.
                                ClaimName: common.GetNamePVC(service.AppDescriptorId,service.ServiceId,fmt.Sprintf("%d",i)),
                            },
                        },
                    }
                }
                volumes = append(volumes,*v)
                vm := &apiv1.VolumeMount{
                    Name: v.Name,
                    MountPath:storage.MountPath,
                }
                volumeMounts = append(volumeMounts, *vm)
            }
            deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes,volumes...)
            deployment.Spec.Template.Spec.Containers[0].VolumeMounts =
                append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
        }


        ztAgentName := fmt.Sprintf("zt-%s",common.FormatName(service.Name))
        ztAgentLabels := map[string]string{
            "agent": "zt-agent",
            "app": service.Labels["app"],
        }
        agent := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: ztAgentName,
                Namespace: d.data.Namespace,
                Labels: map[string] string {
                    utils.NALEJ_ANNOTATION_ORGANIZATION : d.data.OrganizationId,
                    utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : d.data.AppDescriptorId,
                    utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : d.data.AppInstanceId,
                    utils.NALEJ_ANNOTATION_STAGE_ID : d.data.Stage.StageId,
                    utils.NALEJ_ANNOTATION_SERVICE_ID : service.ServiceId,
                    utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID : service.ServiceInstanceId,
                    utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID : d.data.ServiceGroupId,
                    utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID : d.data.ServiceGroupInstanceId,
                    "agent":                                    "zt-agent",
                    "app":                                      service.Labels["app"],
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
                                    "--appInstanceId", d.data.AppInstanceId,
                                    "--appName", d.data.AppName,
                                    "--serviceName", common.FormatName(service.Name),
                                    "--deploymentId", d.data.DeploymentId,
                                    "--fragmentId", d.data.Stage.FragmentId,
                                    "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                    "--organizationId", d.data.OrganizationId,
                                    "--organizationName", d.data.OrganizationName,
                                    "--networkId", d.data.ZtNetworkId,
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
                                LivenessProbe: &apiv1.Probe{
                                    InitialDelaySeconds: 20,
                                    PeriodSeconds:       60,
                                    TimeoutSeconds:      20,
                                    Handler: apiv1.Handler{
                                        Exec: &apiv1.ExecAction{
                                            Command: []string{
                                                "./nalej/zt-agent",
                                                "check",
                                                "--appInstanceId", d.data.AppInstanceId,
                                                "--appName", d.data.AppName,
                                                "--serviceName", common.FormatName(service.Name),
                                                "--deploymentId", d.data.DeploymentId,
                                                "--fragmentId", d.data.Stage.FragmentId,
                                                "--managerAddr", common.DEPLOYMENT_MANAGER_ADDR,
                                                "--organizationId", d.data.OrganizationId,
                                                "--organizationName", d.data.OrganizationName,
                                                "--networkId", d.data.ZtNetworkId,
                                            },
                                        },
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: d.getContainerPorts(service.ExposedPorts),
                                ImagePullPolicy: DefaultImagePullPolicy,
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

        d.deployments = append(d.deployments, DeploymentInfo{service.ServiceId, service.ServiceInstanceId, deployment})
        d.ztAgents = append(d.ztAgents, DeploymentInfo{service.ServiceId, service.ServiceInstanceId, agent})
        //deployments[service.ServiceInstanceId] = deployment
        //agents[service.ServiceInstanceId] = agent
    }


    //d.deployments = deployments
    //d.ztAgents = agents
    return nil
}

func(d *DeployableDeployments) Deploy(controller executor.DeploymentController) error {
    //for serviceId, dep := range d.deployments {
    for _, depInfo := range d.deployments {

        deployed, err := d.client.Create(&depInfo.Deployment)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",depInfo.Deployment.Name)
            return err
        }
        log.Debug().Str("uid",string(deployed.GetUID())).Str("appInstanceID",d.data.AppInstanceId).
            Str("serviceID", depInfo.ServiceId).Str("serviceInstanceId",depInfo.ServiceInstanceId).
            Msg("add nalej deployment resource to be monitored")
        res := entities.NewMonitoredPlatformResource(string(deployed.GetUID()),d.data, depInfo.ServiceId,depInfo.ServiceInstanceId, "")
        controller.AddMonitoredResource(&res)
    }
    // same approach for agents
    //for serviceId, dep := range d.ztAgents {
    for _, depInfo := range d.ztAgents {
        deployed, err := d.client.Create(&depInfo.Deployment)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment for zt-agent %s",depInfo.Deployment.Name)
            return err
        }
        log.Debug().Str("uid",string(deployed.GetUID())).Str("appInstanceID",d.data.AppInstanceId).
            Str("serviceID", depInfo.ServiceId).Str("serviceInstanceId",depInfo.ServiceInstanceId).
            Msg("add zt-agent deployment resource to be monitored")
        res := entities.NewMonitoredPlatformResource(string(deployed.GetUID()),d.data, depInfo.ServiceId, depInfo.ServiceInstanceId, "")
        controller.AddMonitoredResource(&res)
    }
    return nil
}

func(d *DeployableDeployments) Undeploy() error {
    for _, dep := range d.deployments {
        err := d.client.Delete(dep.Deployment.Name,metav1.NewDeleteOptions(DeleteGracePeriod))
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",dep.Deployment.Name)
            return err
        }
    }
    return nil
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
    log.Debug().Interface("nalej_variables", result).Str("appId", d.data.AppInstanceId).Msg("generated variables for service")
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