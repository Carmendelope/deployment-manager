/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
	"encoding/base64"
	"fmt"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeployableSecrets struct {
	client          coreV1.SecretInterface
	stage                 *grpc_conductor_go.DeploymentStage
	targetNamespace string
	secrets      map[string][]*v1.Secret
}

func NewDeployableSecrets(
	client *kubernetes.Clientset,
	stage *grpc_conductor_go.DeploymentStage,
	targetNamespace string) *DeployableSecrets {
	return &DeployableSecrets{
		client:          client.CoreV1().Secrets(targetNamespace),
		stage:           stage,
		targetNamespace: targetNamespace,
		secrets:      make(map[string][]*v1.Secret, 0),
	}
}

func (ds*DeployableSecrets) GetId() string {
	return ds.stage.StageId
}

func (ds*DeployableSecrets) getAuth(username string, password string) string {
	toEncode := fmt.Sprintf("%s:%s", username, password)
	encoded := base64.StdEncoding.EncodeToString([]byte(toEncode))
	return encoded
}

func (ds*DeployableSecrets) getDockerConfigJSON(ic *grpc_application_go.ImageCredentials) string {
	template := "{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}}"
	toEncode := fmt.Sprintf(template, ic.DockerRepository, ic.Username, ic.Password, ic.Email, ds.getAuth(ic.Username, ic.Password))
	return toEncode
}

func (ds*DeployableSecrets) generateDockerSecret(serviceId string, ic *grpc_application_go.ImageCredentials) *v1.Secret {
	return &v1.Secret{
		TypeMeta:   v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:         serviceId,
			Namespace:    ds.targetNamespace,
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(ds.getDockerConfigJSON(ic)),
		},
		Type: v1.SecretTypeDockerConfigJson,
	}
}

// This function returns an array in case we support other secrets in the future.
func (ds*DeployableSecrets) BuildSecretsForService(service *grpc_application_go.Service) []*v1.Secret {
	if service.Credentials == nil{
		return nil
	}
	dockerSecret := ds.generateDockerSecret(service.ServiceId, service.Credentials)
	result := []*v1.Secret{dockerSecret}
	log.Debug().Interface("number", len(result)).Str("serviceName", service.Name).Msg("Secrets prepared for service")
	return result
}

func (ds*DeployableSecrets) Build() error {
	for _, service := range ds.stage.Services {
		toAdd := ds.BuildSecretsForService(service)
		if toAdd != nil && len(toAdd) > 0 {
			ds.secrets[service.ServiceId] = toAdd
		}
	}
	log.Debug().Interface("secrets", ds.secrets).Msg("secrets have been build and are ready to deploy")
	return nil
}

func (ds*DeployableSecrets) Deploy(controller executor.DeploymentController) error {
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
	log.Debug().Int("created", numCreated).Msg("secrets have been created")
	return nil
}

func (ds*DeployableSecrets) Undeploy() error {
	deleted := 0
	for serviceId, secrets := range ds.secrets {
		for _, toDelete := range secrets {
			err := ds.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
			if err != nil {
				log.Error().Str("serviceId", serviceId).Interface("toDelete", toDelete).Msg("cannot delete secret")
				return err

			}
			log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("secrets has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("secrets have been deleted")
	return nil
}


