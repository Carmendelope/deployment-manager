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
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const NalejPublicRegistryName = "nalej-public-registry"

// DeployableNalejSecret represents the 'nalej-public-registry' secret on the user namespaces
// We need DeployableNalejSecret because' nalej-public-registry' secret is special (unique per namespace),
// DeployableSecrets are builded per each stage
type DeployableNalejSecret struct {
	client  coreV1.SecretInterface
	data    entities.DeploymentMetadata
	secrets map[string]*v1.Secret
}

func NewDeployableNalejSecret(
	client *kubernetes.Clientset,
	data entities.DeploymentMetadata) *DeployableNalejSecret {
	return &DeployableNalejSecret{
		client:  client.CoreV1().Secrets(data.Namespace),
		data:    data,
		secrets: map[string]*v1.Secret{},
	}
}

func (ds *DeployableNalejSecret) GetId() string {
	return NalejPublicRegistryName
}

func (ds *DeployableNalejSecret) getAuth(username string, password string) string {
	toEncode := fmt.Sprintf("%s:%s", username, password)
	encoded := base64.StdEncoding.EncodeToString([]byte(toEncode))
	return encoded
}

func (ds *DeployableNalejSecret) getDockerConfigJSON(ic *grpc_application_go.ImageCredentials) string {
	template := "{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}}"
	toEncode := fmt.Sprintf(template, ic.DockerRepository, ic.Username, ic.Password, ic.Email, ds.getAuth(ic.Username, ic.Password))
	return toEncode
}

func (ds *DeployableNalejSecret) BuildNalejPublicRegistry() *v1.Secret {
	return &v1.Secret{
		TypeMeta: v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      NalejPublicRegistryName,
			Namespace: ds.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT: ds.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:     ds.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:      ds.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:     ds.data.AppInstanceId,
			},
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(ds.getDockerConfigJSON(&ds.data.PublicCredentials)),
		},
		Type: v1.SecretTypeDockerConfigJson,
	}
}

func (ds *DeployableNalejSecret) Build() error {
	ds.secrets[NalejPublicRegistryName] = ds.BuildNalejPublicRegistry()
	log.Debug().Interface("Nalej Registry Secret", ds.secrets).Msg("Nalej Registry Secret has been build and are ready to deploy")
	return nil
}

func (ds *DeployableNalejSecret) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for serviceId, toCreate := range ds.secrets {
		log.Debug().Interface("toCreate", toCreate).Msg("creating nalej-public-registry secret")
		created, err := ds.client.Create(toCreate)
		if err != nil {
			log.Error().Err(err).Interface("toCreate", toCreate).Msg("cannot create nalej-public-registry secret")
			return err
		}
		log.Debug().Str("serviceId", serviceId).Str("uid", string(created.GetUID())).Msg("nalej-public-registry secret has been created")
		numCreated++
	}

	log.Debug().Int("created", numCreated).Msg("nalej-public-registry has been created")
	return nil
}

func (ds *DeployableNalejSecret) Undeploy() error {
	deleted := 0
	for serviceId, toDelete := range ds.secrets {
		err := ds.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
		if err != nil {
			log.Error().Str("serviceId", serviceId).Interface("toDelete", toDelete).Msg("cannot delete nalej-public-registry secret")
			return err

		}
		log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("nalej-public-registry secret has been deleted")
	}
	deleted++

	log.Debug().Int("deleted", deleted).Msg("nalej-public-registry Secret have been deleted")
	return nil
}
