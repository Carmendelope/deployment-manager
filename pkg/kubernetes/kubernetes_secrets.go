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
	"encoding/base64"
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type DeployableSecrets struct {
	client       coreV1.SecretInterface
	data         entities.DeploymentMetadata
	secrets      map[string][]*v1.Secret
	planetSecret *v1.Secret
}

func NewDeployableSecrets(
	client *kubernetes.Clientset,
	data entities.DeploymentMetadata) *DeployableSecrets {
	return &DeployableSecrets{
		client:       client.CoreV1().Secrets(data.Namespace),
		data:         data,
		secrets:      make(map[string][]*v1.Secret, 0),
		planetSecret: &v1.Secret{}}
}

func (ds *DeployableSecrets) GetId() string {
	return ds.data.Stage.StageId
}

func (ds *DeployableSecrets) getAuth(username string, password string) string {
	toEncode := fmt.Sprintf("%s:%s", username, password)
	encoded := base64.StdEncoding.EncodeToString([]byte(toEncode))
	return encoded
}

func (ds *DeployableSecrets) getDockerConfigJSON(ic *grpc_application_go.ImageCredentials) string {
	template := "{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}}"
	toEncode := fmt.Sprintf(template, ic.DockerRepository, ic.Username, ic.Password, ic.Email, ds.getAuth(ic.Username, ic.Password))
	return toEncode
}

func (ds *DeployableSecrets) generateDockerSecret(service *grpc_conductor_go.ServiceInstance) *v1.Secret {
	return &v1.Secret{
		TypeMeta: v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      service.ServiceName,
			Namespace: ds.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT:       ds.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:           ds.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:            ds.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:           ds.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:                  ds.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:                service.ServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID:       service.ServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID:          service.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID: service.ServiceGroupInstanceId,
			},
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(ds.getDockerConfigJSON(service.Credentials)),
		},
		Type: v1.SecretTypeDockerConfigJson,
	}
}

// This function returns an array in case we support other Secrets in the future.
func (ds *DeployableSecrets) BuildSecretsForService(service *grpc_conductor_go.ServiceInstance) []*v1.Secret {
	if service.Credentials == nil {
		return nil
	}
	dockerSecret := ds.generateDockerSecret(service)
	result := []*v1.Secret{dockerSecret}
	log.Debug().Interface("number", len(result)).Str("serviceName", service.ServiceName).Msg("Secrets prepared for service")
	return result
}

func (ds *DeployableSecrets) Build() error {
	for _, service := range ds.data.Stage.Services {
		toAdd := ds.BuildSecretsForService(service)
		if toAdd != nil && len(toAdd) > 0 {
			ds.secrets[service.ServiceId] = toAdd
		}
	}

	log.Debug().Interface("Secrets", ds.secrets).Msg("Secrets have been build and are ready to deploy")
	return nil
}

func (ds *DeployableSecrets) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for serviceId, secrets := range ds.secrets {
		for _, toCreate := range secrets {
			log.Debug().Interface("toCreate", toCreate).Msg("creating secret")
			created, err := ds.client.Create(toCreate)
			if err != nil {
				log.Error().Err(err).Interface("toCreate", toCreate).Msg("cannot create secret")
				return err
			}
			log.Debug().Str("serviceId", serviceId).Str("uid", string(created.GetUID())).Msg("secret has been created")
			numCreated++
		}
	}
	return nil
}

func (ds *DeployableSecrets) Undeploy() error {
	deleted := 0
	for serviceId, secrets := range ds.secrets {
		for _, toDelete := range secrets {
			err := ds.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
			if err != nil {
				log.Error().Str("serviceId", serviceId).Interface("toDelete", toDelete).Msg("cannot delete secret")
				return err

			}
			log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("Secrets has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("Secrets have been deleted")
	return nil
}
