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
	// Prefix defining Nalej Services
	NalejServicePrefix = "NALEJ_SERV_"
	// Default imagePullPolicy
	DefaultImagePullPolicy = apiv1.PullAlways
	// Default storage size
	DefaultStorageAllocationSize = int64(100 * 1024 * 1024)
)

// Deployable Deployments
//-----------------------

type DeployableDeployments struct {
	// kubernetes Client
	Client v1.DeploymentInterface
	// stage metadata
	Data entities.DeploymentMetadata
	// array of Deployments ready to be deployed
	// [[service_id, service_instance_id, deployment],...]
	Deployments []*appsv1.Deployment
	// network decorator object for deployments
	networkDecorator executor.NetworkDecorator
}

func NewDeployableDeployment(
	client *kubernetes.Clientset, data entities.DeploymentMetadata, networkDecorator executor.NetworkDecorator) *DeployableDeployments {
	return &DeployableDeployments{
		Client:      client.AppsV1().Deployments(data.Namespace),
		Data:        data,
		Deployments: make([]*appsv1.Deployment, 0),
		networkDecorator: networkDecorator,
	}
}

// NewDeployableDeploymentForTest creates a new empty DeployableDeployment (only for TEST!)
func NewDeployableDeploymentForTest() *DeployableDeployments {
	return &DeployableDeployments{
		Deployments: make([]*appsv1.Deployment, 0),
	}
}

func (d *DeployableDeployments) GetId() string {
	return d.Data.Stage.StageId
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
	if res[len(res)-1] == '/' {
		res = res[:len(res)-1]
	}

	return ReformatLabel(res)

}

func (d *DeployableDeployments) generateAllVolumes(serviceId string, serviceInstanceId string, configFiles []*grpc_application_go.ConfigFile) ([]apiv1.Volume, []apiv1.VolumeMount) {
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
		if !exists {
			volume := apiv1.Volume{
				Name: clearPath,
				VolumeSource: apiv1.VolumeSource{
					ConfigMap: &apiv1.ConfigMapVolumeSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: fmt.Sprintf("config-map-%s-%s", serviceId, serviceInstanceId),
						},
						Items: []apiv1.KeyToPath{{
							Key:  config.ConfigFileId,
							Path: file,
						},
						},
					},
				},
			}
			volumes = append(volumes, volume)
			pathAdded[clearPath] = true

			volumeMount := apiv1.VolumeMount{
				Name:      clearPath,
				ReadOnly:  true,
				MountPath: path,
			}
			volumesMount = append(volumesMount, volumeMount)

		} else { // if we has found this path before -> we only have to add a key-path in the volumes (in its corresponding entry)
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

func (d *DeployableDeployments) Build() error {

	for serviceIndex, service := range d.Data.Stage.Services {
		log.Debug().Msgf("build deployment %s %d out of %d", service.ServiceId, serviceIndex+1, len(d.Data.Stage.Services))

		var extendedLabels map[string]string
		if service.Labels != nil {
			// users have already defined labels for this app
			extendedLabels = service.Labels
		} else {
			extendedLabels = make(map[string]string, 0)
		}


		extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.Data.FragmentId
		extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = d.Data.OrganizationId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.Data.AppDescriptorId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.Data.AppInstanceId
		extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = d.Data.Stage.StageId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
		extendedLabels[utils.NALEJ_ANNOTATION_IS_PROXY] = "false"

		environmentVariables := d.getEnvVariables(d.Data.NalejVariables, service.EnvironmentVariables)
		environmentVariables = d.addDeviceGroupEnvVariables(environmentVariables, service.ServiceGroupInstanceId, service.ServiceInstanceId)

		deployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.FormatName(service.ServiceName),
				Namespace: d.Data.Namespace,
				Labels:    extendedLabels,
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
						Containers: []apiv1.Container{
							// User defined container
							{
								Name:            common.FormatName(service.ServiceName),
								Image:           service.Image,
								Env:             environmentVariables,
								Ports:           d.getContainerPorts(service.ExposedPorts),
								ImagePullPolicy: DefaultImagePullPolicy,
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
					Name: service.ServiceName,
				})
		}

		if service.RunArguments != nil {
			log.Debug().Msg("Adding arguments")
			deployment.Spec.Template.Spec.Containers[0].Args = service.RunArguments
		}

		if service.Configs != nil && len(service.Configs) > 0 {
			log.Debug().Msg("Adding config maps")
			log.Debug().Msg("Creating volumes")
			configVolumes, cmVolumeMounts := d.generateAllVolumes(service.ServiceId, service.ServiceInstanceId, service.Configs)
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, configVolumes...)
			log.Debug().Msg("Linking configmap volumes")
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = cmVolumeMounts
		}
		if service.Storage != nil && len(service.Storage) > 0 {
			// Set VolumeMounts and Volumes based on storage type
			volumes := make([]apiv1.Volume, 0)
			volumeMounts := make([]apiv1.VolumeMount, 0)

			for i, storage := range service.Storage {
				if storage.Size == 0 {
					storage.Size = DefaultStorageAllocationSize
				}
				var v *apiv1.Volume
				if storage.Type == grpc_application_go.StorageType_EPHEMERAL {
					v = &apiv1.Volume{
						Name: fmt.Sprintf("vol-1%d", i),
						VolumeSource: apiv1.VolumeSource{
							EmptyDir: &apiv1.EmptyDirVolumeSource{
								Medium:    apiv1.StorageMediumDefault,
								SizeLimit: resource.NewQuantity(storage.Size, resource.BinarySI),
							},
						},
					}
				} else {
					v = &apiv1.Volume{
						Name: fmt.Sprintf("vol-1%d", i),
						VolumeSource: apiv1.VolumeSource{
							PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
								// claim name should be same as pvcID that was generated in BuildStorageForServices.
								// TODO Check storage attached to replicas
								ClaimName: common.GeneratePVCName(service.ServiceGroupInstanceId, service.ServiceId, fmt.Sprintf("%d", i)),
							},
						},
					}
				}
				volumes = append(volumes, *v)
				vm := &apiv1.VolumeMount{
					Name:      v.Name,
					MountPath: storage.MountPath,
				}
				volumeMounts = append(volumeMounts, *vm)
			}
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volumes...)
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts =
				append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
		}

		d.Deployments = append(d.Deployments, &deployment)
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := d.networkDecorator.Build(d)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error building network components")
		return errNetDecorator
	}

	return nil
}


func (d *DeployableDeployments) Deploy(controller executor.DeploymentController) error {

	// same approach for service Deployments
	for _, deployment := range d.Deployments {

		deployed, err := d.Client.Create(deployment)
		if err != nil {
			log.Error().Interface("deployment", deployment).
				Err(err).Msgf("error creating deployment %s", deployment.Name)
			return err
		}
		log.Debug().Str("uid", string(deployed.GetUID())).Str("appInstanceID", d.Data.AppInstanceId).
			Str("serviceID", deployment.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID]).
			Str("serviceInstanceId", deployment.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]).
			Msg("add nalej deployment resource to be monitored")
		res := entities.NewMonitoredPlatformResource(deployed.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT], string(deployed.GetUID()),
			deployed.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], deployed.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
			deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
			deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], deployed.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
		controller.AddMonitoredResource(&res)
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := d.networkDecorator.Build(d)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error running networking decorator during deployment deploy")
		return errNetDecorator
	}

	return nil
}

func (d *DeployableDeployments) Undeploy() error {
	for _, dep := range d.Deployments {
		err := d.Client.Delete(dep.Name, metav1.NewDeleteOptions(DeleteGracePeriod))
		if err != nil {
			log.Error().Err(err).Msgf("error creating deployment %s", dep.Name)
			return err
		}
	}
	return nil
}

func (d *DeployableDeployments) addDeviceGroupEnvVariables(previous []apiv1.EnvVar, serviceGroupInstanceId string, serviceInstanceId string) []apiv1.EnvVar {
	for _, sr := range d.Data.Stage.DeviceGroupRules {
		if sr.TargetServiceGroupInstanceId == serviceGroupInstanceId && sr.TargetServiceInstanceId == serviceInstanceId {
			toAdd := &apiv1.EnvVar{
				Name:  utils.NALEJ_ANNOTATION_DG_SECRETS,
				Value: strings.Join(sr.DeviceGroupJwtSecrets, ","),
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
	// The cluster id is enabled by default
	result = append(result, apiv1.EnvVar{Name: utils.NALEJ_ENV_CLUSTER_ID, Value: config.GetConfig().ClusterId})

	for k, v := range variables {
		if strings.HasPrefix(k, NalejServicePrefix) {
			// The key cannot have the NalejServicePrefix, that is a reserved word
			log.Warn().Str("UserEnvironmentVariable", k).Msg("reserved variable name will be ignored")
		} else {
			newValue := v
			// check if we have to replace a NALEJ_SERVICE variable
			for nalejK, nalejVariable := range nalejVariables {
				newValue = strings.Replace(newValue, nalejK, nalejVariable, -1)
			}
			toAdd := apiv1.EnvVar{Name: k, Value: newValue}
			log.Debug().Str("formerKey", k).Str("formerValue", v).Interface("apiv1EnvVar", toAdd).Msg("environmentVariable")
			result = append(result, toAdd)
		}
	}
	for k, v := range nalejVariables {
		result = append(result, apiv1.EnvVar{Name: k, Value: v})
	}
	log.Debug().Interface("nalej_variables", result).Str("appId", d.Data.AppInstanceId).Msg("generated variables for service")
	return result
}

// Transform a Nalej list of exposed ports into a K8s api port.
//  params:
//   ports list of exposed ports
//  return:
//   list of ports into k8s api format
func (d *DeployableDeployments) getContainerPorts(ports []*pbApplication.Port) []apiv1.ContainerPort {
	obtained := make([]apiv1.ContainerPort, 0, len(ports))
	for _, p := range ports {
		obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name})
	}
	return obtained
}

