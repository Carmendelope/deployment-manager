/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
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
	"io/ioutil"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type DeployableSecrets struct {
	client          coreV1.SecretInterface
	planetPath      string
	data            entities.DeploymentMetadata
	secrets         map[string][]*v1.Secret
	planetSecret    *v1.Secret
}

func NewDeployableSecrets(
	client      *kubernetes.Clientset,
	planetPath  string,
	data        entities.DeploymentMetadata) *DeployableSecrets {
	return &DeployableSecrets{
		client:          client.CoreV1().Secrets(data.Namespace),
		planetPath:      planetPath,
		data:            data,
		secrets:         make(map[string][]*v1.Secret, 0),
		planetSecret:    &v1.Secret{}}
}

func (ds*DeployableSecrets) GetId() string {
	return ds.data.Stage.StageId
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

func (ds*DeployableSecrets) generateDockerSecret(service *grpc_application_go.ServiceInstance) *v1.Secret {
	return &v1.Secret{
		TypeMeta:   v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:         service.Name,
			Namespace:    ds.data.Namespace,
			Labels: map[string]string {
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT : ds.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION : ds.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : ds.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : ds.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID : ds.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID : service.ServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID : service.ServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID : service.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID : service.ServiceGroupInstanceId,
			},
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(ds.getDockerConfigJSON(service.Credentials)),
		},
		Type: v1.SecretTypeDockerConfigJson,
	}
}

func (ds *DeployableSecrets) generatePlanetSecret (namespace string) *v1.Secret {
	log.Debug().Interface("Secrets", ds.planetSecret).Msg("Creating ZT Planet secret")
	planetData, err := ioutil.ReadFile(ds.planetPath)
	if err != nil {
		log.Error().Msg("cannot read planet file")
		return nil
	}
	return &v1.Secret{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:         ZTPlanetSecretName,
			GenerateName: "",
			Namespace:    namespace,
			Labels: map[string]string {
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT : ds.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION : ds.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : ds.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : ds.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID : ds.data.Stage.StageId,
			},
		},
		Data: map[string][]byte{
			"planet": planetData,
		},
		Type: v1.SecretTypeOpaque,
	}
}

// This function returns an array in case we support other Secrets in the future.
func (ds*DeployableSecrets) BuildSecretsForService(service *grpc_application_go.ServiceInstance) []*v1.Secret {
	if service.Credentials == nil{
		return nil
	}
	dockerSecret := ds.generateDockerSecret(service)
	result := []*v1.Secret{dockerSecret}
	log.Debug().Interface("number", len(result)).Str("serviceName", service.Name).Msg("Secrets prepared for service")
	return result
}


func (ds*DeployableSecrets) Build() error {
	for _, service := range ds.data.Stage.Services {
		toAdd := ds.BuildSecretsForService(service)
		if toAdd != nil && len(toAdd) > 0 {
			ds.secrets[service.ServiceId] = toAdd
		}
	}

	ds.planetSecret = ds.generatePlanetSecret(ds.data.Namespace)

	log.Debug().Interface("Secrets", ds.secrets).Msg("Secrets have been build and are ready to deploy")
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

	_, planetCheck := ds.client.Get(ZTPlanetSecretName, metaV1.GetOptions{})

	if  planetCheck != nil {
		_, err := ds.client.Create(ds.planetSecret)
		if err != nil {
			log.Error().Err(err).Interface("toCreate", ds.planetSecret).Msg("cannot create planet secret")
			return err
		}
		log.Debug().Int("created", numCreated).Msg("Secrets have been created")
	} else {
		log.Warn().Err(planetCheck).Interface("toCreate", ds.planetSecret).Msg("planet secret already created")
	}

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
			log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("Secrets has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("Secrets have been deleted")
	return nil
}


