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
    "github.com/nalej/deployment-manager/pkg/config"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/utils"
    "github.com/nalej/grpc-application-go"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/rs/zerolog/log"
    appsv1 "k8s.io/api/apps/v1"
    apiv1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/typed/apps/v1"
    "strings"
)

const (
    // Name of the Docker ZT agent image
    ZTAgentImageName = "nalejpublic.azurecr.io/nalej/zt-agent:v0.3.0"
    // Prefix defining Nalej Services
    NalejServicePrefix = "NALEJ_SERV_"
    // Default imagePullPolicy
    DefaultImagePullPolicy = apiv1.PullAlways
    //Default storage size
    DefaultStorageAllocationSize = int64(100*1024*1024)
    // zt-planet secret name
    ZTPlanetSecretName = "zt-planet"
    // Default Nalej public registry
    DefaultNalejPublicRegistry = "nalej-public-registry"
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
// NewDeployableDeploymentForTest creates a new empty DeployableDeployment (only for TEST!)
func NewDeployableDeploymentForTest() *DeployableDeployments {
    return &DeployableDeployments{
        deployments: make([]DeploymentInfo,0),
        ztAgents: make([]DeploymentInfo,0),
    }
}

func(d *DeployableDeployments) GetId() string {
    return d.data.Stage.StageId
}
// NP-694. Support consolidating config maps
// createVolumeName transform a path into a name deleting '/' from the end and from the beginning
// and replacing the '/' character for '-'
func createVolumeName(name string) string {
    if name == "" {
        return ""
    }

    var res string
    res = strings.TrimSpace(name)

    if res[0] == '/' {
        res = res[1:]
    }
    if res [len(res)-1] == '/' {
        res = res[:len(res)-1]
    }

    return ReformatLabel(res)

}

func (d *DeployableDeployments) generateAllVolumes (serviceId string, serviceInstanceId string, configFiles []*grpc_application_go.ConfigFile) ([]apiv1.Volume, []apiv1.VolumeMount) {
    if configFiles == nil {
        return nil, nil
    }

    volumes := make([]apiv1.Volume, 0)
    pathAdded := make(map[string]bool, 0)
    volumesMount := make([]apiv1.VolumeMount, 0)

    for _, config := range configFiles {
        path, file := GetConfigMapPath(config.MountPath)
        clearPath := createVolumeName(path)
        _, exists := pathAdded[clearPath]
        // if we has not found this path before -> create a volumeMount and volume entries
        if ! exists {
            volume := apiv1.Volume{
                Name: clearPath,
                VolumeSource: apiv1.VolumeSource{
                    ConfigMap: &apiv1.ConfigMapVolumeSource {
                        LocalObjectReference: apiv1.LocalObjectReference{
                            Name: fmt.Sprintf("config-map-%s-%s", serviceId, serviceInstanceId),
                        },
                        Items: []apiv1.KeyToPath{{
                            Key: config.ConfigFileId,
                            Path: file,
                        },
                        },
                    },
                },
            }
            volumes = append(volumes, volume)
            pathAdded[clearPath] = true

            volumeMount := apiv1.VolumeMount{
                Name: clearPath,
                ReadOnly:true,
                MountPath: path,
            }
            volumesMount = append(volumesMount, volumeMount)

        }else { // if we has found this path before -> we only have to add a key-path in the volumes (in its corresponding entry)
            // find the volumeMount and add a Key_Path
            newKeyPath := apiv1.KeyToPath{
                Key:  config.ConfigFileId,
                Path: file,
            }
            for i := 0; i < len(volumes); i++ {
                if volumes[i].Name == clearPath {
                    volumes[i].VolumeSource.ConfigMap.Items = append(volumes[i].VolumeSource.ConfigMap.Items, newKeyPath)
                    break
                }
            }
        }
    }
    return volumes, volumesMount
}


func(d *DeployableDeployments) Build() error {

    for serviceIndex, service := range d.data.Stage.Services {
        log.Debug().Msgf("build deployment %s %d out of %d", service.ServiceId,serviceIndex+1,len(d.data.Stage.Services))

        // value for privileged user
        user0 := int64(0)
        privilegedUser := &user0

        var extendedLabels map[string]string
        if service.Labels != nil {
            // users have already defined labels for this app
            extendedLabels = service.Labels
        } else {
            extendedLabels = make(map[string]string,0)
        }
        extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.data.FragmentId
        extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = d.data.OrganizationId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.data.AppDescriptorId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.data.AppInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = d.data.Stage.StageId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_IS_PROXY] = "false"

        environmentVariables := d.getEnvVariables(d.data.NalejVariables,service.EnvironmentVariables)
        environmentVariables = d.addDeviceGroupEnvVariables(environmentVariables, service.ServiceGroupInstanceId, service.ServiceInstanceId)

        deployment := appsv1.Deployment{
            ObjectMeta: metav1.ObjectMeta{
                Name: common.FormatName(service.Name),
                Namespace: d.data.Namespace,
                Labels: extendedLabels,
            },
            Spec: appsv1.DeploymentSpec{
                Replicas: int32Ptr(service.Specs.Replicas),
                Selector: &metav1.LabelSelector{
                    MatchLabels: extendedLabels,
                },
                Template: apiv1.PodTemplateSpec{
                    ObjectMeta: metav1.ObjectMeta{
                        Labels: extendedLabels,
                    },
                    Spec: apiv1.PodSpec{
                        // Do not inject K8S service names
                        EnableServiceLinks: getBool(false),
                        // Do not mount any service account token
                        AutomountServiceAccountToken: getBool(false),
                        // Set POD DNS policies
                        DNSPolicy: apiv1.DNSNone,
                        DNSConfig: &apiv1.PodDNSConfig{
                            Nameservers: d.data.DNSHosts,
                        },
                        ImagePullSecrets: []apiv1.LocalObjectReference{
                          {
                            Name: DefaultNalejPublicRegistry,
                          },
                        },
                        Containers: []apiv1.Container{
                            // User defined container
                            {
                                Name:  common.FormatName(service.Name),
                                Image: service.Image,
                                Env:   environmentVariables,
                                Ports: d.getContainerPorts(service.ExposedPorts, false),
                                ImagePullPolicy: DefaultImagePullPolicy,
                            },
                            // ZT sidecar container
                            {
                                Name: "zt-sidecar",
                                Image: ZTAgentImageName,
                                Args: []string{
                                    "run",
                                },
                                Env: d.getContainerEnvVariables(service, false),
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
                                                "--managerAddr", config.GetConfig().DeploymentMgrAddress,
                                                "--organizationId", d.data.OrganizationId,
                                                "--organizationName", d.data.OrganizationName,
                                                "--networkId", d.data.ZtNetworkId,
                                                "--serviceGroupInstanceId", service.ServiceGroupInstanceId,
                                                "--serviceAppInstanceId", service.ServiceInstanceId,
                                            },
                                        },
                                    },
                                },
                                // The proxy exposes the same ports of the deployment
                                Ports: d.getContainerPorts(service.ExposedPorts, true),
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
                                    // volume mount for the zt-planet secret
                                    {
                                        Name: ZTPlanetSecretName,
                                        MountPath: "/zt/planet",
                                        ReadOnly: true,
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
                            // zt-planet secret
                            {
                                Name: ZTPlanetSecretName,
                                VolumeSource: apiv1.VolumeSource{
                                    Secret: &apiv1.SecretVolumeSource{
                                        SecretName: ZTPlanetSecretName,
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
            deployment.Spec.Template.Spec.ImagePullSecrets = append(deployment.Spec.Template.Spec.ImagePullSecrets,
                apiv1.LocalObjectReference{
                    Name: service.ServiceId,
                })
        }

        if service.RunArguments != nil {
            log.Debug().Msg("Adding arguments")
            deployment.Spec.Template.Spec.Containers[0].Args = service.RunArguments
        }

        if service.Configs != nil && len(service.Configs) > 0 {
            log.Debug().Msg("Adding config maps")
            log.Debug().Msg("Creating volumes")
            // NP-694. Support consolidating config maps
            configVolumes, cmVolumeMounts := d.generateAllVolumes(service.ServiceId, service.ServiceInstanceId, service.Configs)
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
                                // TODO Check storage attached to replicas
                                ClaimName: common.GeneratePVCName(service.ServiceGroupInstanceId,service.ServiceId,fmt.Sprintf("%d",i)),
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

        d.deployments = append(d.deployments, DeploymentInfo{service.ServiceId, service.ServiceInstanceId, deployment})

        // If we have at least one single port defined there will be a K8s service. Create the corresponding proxy agent
        if len(service.ExposedPorts) != 0 {
            agent := d.createZtAgent(service,extendedLabels)
            d.ztAgents = append(d.ztAgents, DeploymentInfo{service.ServiceId, service.ServiceInstanceId, agent})
        }

    }

    return nil
}


// Private helper function to generate zt agents
func (d *DeployableDeployments) createZtAgent(service *grpc_application_go.ServiceInstance, extendedLabels map[string]string)  appsv1.Deployment {
    // value for privileged user
    user0 := int64(0)
    privilegedUser := &user0

    ztAgentName := fmt.Sprintf("zt-%s",common.FormatName(service.Name))
    // The proxy has the same labels with the proxy flag activated
    // copy the map and modify the proxy flag
    ztAgentLabels := make(map[string]string,0)
    for k,v := range extendedLabels {
        ztAgentLabels[k] = v
    }
    ztAgentLabels[utils.NALEJ_ANNOTATION_IS_PROXY] = "true"

    agent := appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name: ztAgentName,
            Namespace: d.data.Namespace,
            Labels: ztAgentLabels,
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
                    // Do not mount any service account token
                    AutomountServiceAccountToken: getBool(false),
                    Containers: []apiv1.Container{
                        // zero-tier sidecar
                        {
                            Name: ztAgentName,
                            Image: ZTAgentImageName,
                            Args: []string{
                                "run",
                            },
                            Env: d.getContainerEnvVariables(service, true),
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
                                            "--managerAddr", config.GetConfig().DeploymentMgrAddress,
                                            "--organizationId", d.data.OrganizationId,
                                            "--organizationName", d.data.OrganizationName,
                                            "--networkId", d.data.ZtNetworkId,
                                            "--serviceGroupInstanceId", service.ServiceGroupInstanceId,
                                            "--serviceAppInstanceId", service.ServiceInstanceId,
                                        },
                                    },
                                },
                            },
                            // The proxy exposes the same ports of the deployment
                            Ports: d.getContainerPorts(service.ExposedPorts, true),
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
                                // volume mount for the zt-planet secret
                                {
                                    Name: ZTPlanetSecretName,
                                    MountPath: "/zt/planet",
                                    ReadOnly: true,
                                },
                            },
                        },
                    },
                    ImagePullSecrets: []apiv1.LocalObjectReference{
                        {
                            Name: DefaultNalejPublicRegistry,
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
                        // zt-planet secret
                        {
                            Name: ZTPlanetSecretName,
                            VolumeSource: apiv1.VolumeSource{
                                Secret: &apiv1.SecretVolumeSource{
                                    SecretName: ZTPlanetSecretName,
                                },
                            },
                        },
                    },
                },
            },
        },
    }
    return agent
}

func(d *DeployableDeployments) Deploy(controller executor.DeploymentController) error {

    for _, depInfo := range d.ztAgents {
        deployed, err := d.client.Create(&depInfo.Deployment)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment for zt-agent %s",depInfo.Deployment.Name)
            return err
        }
        log.Debug().Str("uid",string(deployed.GetUID())).Str("appInstanceID",d.data.AppInstanceId).
            Str("serviceID", depInfo.ServiceId).Str("serviceInstanceId",depInfo.ServiceInstanceId).
            Msg("add zt-agent deployment resource to be monitored")
        res := entities.NewMonitoredPlatformResource(deployed.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],string(deployed.GetUID()),
            deployed.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], deployed.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
            deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
            deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
        controller.AddMonitoredResource(&res)
    }

    // same approach for service deployments
    for _, depInfo := range d.deployments {

        deployed, err := d.client.Create(&depInfo.Deployment)
        if err != nil {
            log.Error().Interface("deployment", depInfo.Deployment).Err(err).Msgf("error creating deployment %s",depInfo.Deployment.Name)
            return err
        }
        log.Debug().Str("uid",string(deployed.GetUID())).Str("appInstanceID",d.data.AppInstanceId).
            Str("serviceID", depInfo.ServiceId).Str("serviceInstanceId",depInfo.ServiceInstanceId).
            Msg("add nalej deployment resource to be monitored")
        res := entities.NewMonitoredPlatformResource(deployed.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],string(deployed.GetUID()),
            deployed.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], deployed.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
            deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
            deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
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

func(d * DeployableDeployments) addDeviceGroupEnvVariables(previous []apiv1.EnvVar, serviceGroupInstanceId string, serviceInstanceId string) []apiv1.EnvVar {
    for _, sr := range d.data.Stage.DeviceGroupRules{
        if sr.TargetServiceGroupInstanceId == serviceGroupInstanceId && sr.TargetServiceInstanceId == serviceInstanceId{
            toAdd := &apiv1.EnvVar{
                Name:      utils.NALEJ_ANNOTATION_DG_SECRETS,
                Value:     strings.Join(sr.DeviceGroupJwtSecrets,","),
            }
            log.Debug().Interface("envVar", toAdd).Interface("sr", sr).Msg("Adding a new environment variable for security groups")
            previous = append(previous, *toAdd)
            return previous
        }
    }
    return previous
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
func (d *DeployableDeployments) getContainerPorts(ports []*pbApplication.Port, isSidecar bool) []apiv1.ContainerPort {
    obtained := make([]apiv1.ContainerPort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name})
    }
    if isSidecar{
        obtained = append(obtained, apiv1.ContainerPort{ContainerPort:int32(config.GetConfig().ZTSidecarPort), Name: "ztrouteport"})
    }
    return obtained
}

// Generate a set of environment variables for a container. This private function is particularly designed to
// support the building of proxies.
// params:
//  service specification to be deployed
//  isProxy boolean value indicating whether the environment variables to be created correspond to a proxy
// return:
//  slice of api environment variables
func (d *DeployableDeployments) getContainerEnvVariables(service *pbApplication.ServiceInstance, isProxy bool) []apiv1.EnvVar{
    return []apiv1.EnvVar{
        {
            Name: utils.NALEJ_ENV_CLUSTER_ID, Value: config.GetConfig().ClusterId,
        },
        {
            Name: utils.NALEJ_ENV_ZT_NETWORK_ID, Value: d.data.ZtNetworkId,
        },
        {
            Name: utils.NALEJ_ENV_IS_PROXY, Value: fmt.Sprintf("%t",isProxy),
        },
        {
            Name: utils.NALEJ_ENV_MANAGER_ADDR, Value: config.GetConfig().DeploymentMgrAddress,
        },
        {
            Name: utils.NALEJ_ENV_DEPLOYMENT_ID, Value: d.data.DeploymentId,
        },
        {
            Name: utils.NALEJ_ENV_DEPLOYMENT_FRAGMENT, Value: d.data.Stage.FragmentId,
        },
        {
            Name: utils.NALEJ_ENV_ORGANIZATION_ID, Value: d.data.OrganizationId,
        },
        {
            Name: utils.NALEJ_ENV_ORGANIZATION_NAME, Value: d.data.OrganizationName,
        },
        {
            Name: utils.NALEJ_ENV_APP_DESCRIPTOR, Value: d.data.AppDescriptorId,
        },
        {
            Name: utils.NALEJ_ENV_APP_NAME, Value: d.data.AppName,
        },
        {
            Name: utils.NALEJ_ENV_APP_INSTANCE_ID, Value: d.data.AppInstanceId,
        },
        {
            Name: utils.NALEJ_ENV_STAGE_ID, Value: d.data.Stage.StageId,
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_NAME, Value: service.Name,
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_ID, Value: service.ServiceId,
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_FQDN, Value: common.GetServiceFQDN(service.Name, service.OrganizationId, service.AppInstanceId),
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_INSTANCE_ID, Value: service.ServiceInstanceId,
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_GROUP_ID, Value: service.ServiceGroupId,
        },
        {
            Name: utils.NALEJ_ENV_SERVICE_GROUP_INSTANCE_ID, Value: service.ServiceGroupInstanceId,
        },
    }
}