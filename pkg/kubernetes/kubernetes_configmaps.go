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
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/grpc-application-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"strings"
)

type DeployableConfigMaps struct {
	client     coreV1.ConfigMapInterface
	data       entities.DeploymentMetadata
	configmaps map[string][]*v1.ConfigMap
}

// NewDeployableConfigMapsForTest creates an empty DeployableConfigMaps (ONLY FOR TESTS!!)
func NewDeployableConfigMapsForTest(data entities.DeploymentMetadata) *DeployableConfigMaps {
	return &DeployableConfigMaps{
		client:     nil,
		data:       data,
		configmaps: make(map[string][]*v1.ConfigMap, 0),
	}
}

func NewDeployableConfigMaps(
	client *kubernetes.Clientset,
	data entities.DeploymentMetadata) *DeployableConfigMaps {
	return &DeployableConfigMaps{
		client:     client.CoreV1().ConfigMaps(data.Namespace),
		data:       data,
		configmaps: make(map[string][]*v1.ConfigMap, 0),
	}
}

func (dc *DeployableConfigMaps) GetId() string {
	return dc.data.Stage.StageId
}

func GetConfigMapPath(mountPath string) (string, string) {
	index := strings.LastIndex(mountPath, "/")
	if index == -1 {
		return "/", mountPath
	}
	return mountPath[0 : index+1], mountPath[index+1:]
}

/*
// TODO this code seems to be unused. Remove it if proceeds
func (dc *DeployableConfigMaps) generateConfigMap(serviceId string, serviceInstanceId string, cf *grpc_application_go.ConfigFile) *v1.ConfigMap {
	log.Debug().Interface("configMap", cf).Msg("generating config map...")
	_, file := GetConfigMapPath(cf.MountPath)
	log.Debug().Str("file", file).Msg("Config map content")
	return &v1.ConfigMap{
		TypeMeta: v12.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      cf.ConfigFileId,
			Namespace: dc.data.Namespace,
			Labels:    map[string]string{
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID : dc.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : dc.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : dc.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID : dc.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID : serviceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID : serviceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID : dc.data.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID : dc.data.ServiceGroupInstanceId,
			},
		},
		BinaryData: map[string][]byte{
			file: cf.Content,
		},
	}
}
*/

func (dc *DeployableConfigMaps) generateConsolidateConfigMap(service *grpc_application_go.ServiceInstance, cf []*grpc_application_go.ConfigFile) *v1.ConfigMap {
	log.Debug().Interface("configMap", cf).Msg("generating consolidate config map...")

	if len(cf) == 0 {
		return nil
	}

	binaryData := make(map[string][]byte, 0)

	for _, config := range cf {
		binaryData[config.ConfigFileId] = config.Content
	}

	return &v1.ConfigMap{
		TypeMeta: v12.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      fmt.Sprintf("config-map-%s-%s", service.ServiceId, service.ServiceInstanceId),
			Namespace: dc.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT:       dc.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:           dc.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:            dc.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:           dc.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:                  dc.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:                service.ServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID:       service.ServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID:          service.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID: service.ServiceGroupInstanceId,
			},
		},
		BinaryData: binaryData,
	}

	return nil
}

func (dc *DeployableConfigMaps) Build() error {
	for _, service := range dc.data.Stage.Services {
		//toAdd := dc.generateConsolidateConfigMap(service.ServiceId, service.ServiceInstanceId, service.Configs)
		toAdd := dc.generateConsolidateConfigMap(service, service.Configs)
		if toAdd != nil {
			log.Debug().Interface("toAdd", toAdd).Str("serviceName", service.Name).Msg("Adding new config file")
			dc.configmaps[service.ServiceId] = append(dc.configmaps[service.ServiceId], toAdd)
		}
	}
	log.Debug().Interface("Configmaps", dc.configmaps).Msg("configmap have been build and are ready to deploy")
	return nil
}

func (dc *DeployableConfigMaps) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for serviceId, configmaps := range dc.configmaps {
		for _, toCreate := range configmaps {
			log.Debug().Interface("toCreate", toCreate).Msg("creating config map")
			created, err := dc.client.Create(toCreate)
			if err != nil {
				log.Error().Err(err).Interface("toCreate", toCreate).Msg("cannot create config map")
				return err
			}
			log.Debug().Str("serviceId", serviceId).Str("uid", string(created.GetUID())).Msg("Configmap has been created")
			numCreated++
		}
	}
	log.Debug().Int("created", numCreated).Msg("Configmaps have been created")
	return nil
}

func (dc *DeployableConfigMaps) Undeploy() error {
	deleted := 0
	for serviceId, configmaps := range dc.configmaps {
		for _, toDelete := range configmaps {
			err := dc.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
			if err != nil {
				log.Error().Str("serviceId", serviceId).Interface("toDelete", toDelete).Msg("cannot delete configmap")
				return err

			}
			log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("Configmap has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("Configmaps have been deleted")
	return nil
}
