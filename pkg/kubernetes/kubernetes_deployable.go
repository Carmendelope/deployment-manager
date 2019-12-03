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
	pbApplication "github.com/nalej/grpc-application-go"
	"github.com/rs/zerolog/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

/*
 * Specification of potential k8s deployable resources and their functions.
 */

const (
	// Grace period in seconds to delete a deployable.
	DeleteGracePeriod = 10
)

// Definition of a collection of deployable resources contained by a stage. This object is deployable and it has
// deployable objects itself. The deploy of this object simply consists on the deployment of the internal objects.
type DeployableKubernetesStage struct {
	// kubernetes Client
	client *kubernetes.Clientset
	// deployment Data
	data entities.DeploymentMetadata
	// collection of Deployments
	Deployments *DeployableDeployments
	// collection of Services
	Services *DeployableServices
	// Collection of Ingresses to be deployed
	Ingresses *DeployableIngress
	// Collection of maps to be deployed.
	Configmaps *DeployableConfigMaps
	// Collection of Secrets to be deployed.
	Secrets *DeployableSecrets
	// Collection of persistence Volume claims
	Storage *DeployableStorage
	// Collection of endpoints related to device groups.
	DeviceGroupServices *DeployableDeviceGroups
	// Collection of load balancers
	LoadBalancers *DeployableLoadBalancer
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the Namespace the resources will be deployed into
func NewDeployableKubernetesStage(
	client *kubernetes.Clientset, data entities.DeploymentMetadata,
	networkDecorator executor.NetworkDecorator) *DeployableKubernetesStage {
	return &DeployableKubernetesStage{
		client:              client,
		data:                data,
		Services:            NewDeployableService(client, data, networkDecorator),
		Deployments:         NewDeployableDeployment(client, data, networkDecorator),
		Ingresses:           NewDeployableIngress(client, data, networkDecorator),
		Configmaps:          NewDeployableConfigMaps(client, data),
		Secrets:             NewDeployableSecrets(client, data),
		Storage:             NewDeployableStorage(client, data),
		DeviceGroupServices: NewDeployableDeviceGroups(client, data),
		LoadBalancers:       NewDeployableLoadBalancer(client, data),
	}
}

func (d DeployableKubernetesStage) GetId() string {
	return d.data.Stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
	// Build Deployments
	err := d.Deployments.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create Deployments")
		return err
	}
	// Build Services
	err = d.Services.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create Services for")
		return err
	}
	err = d.DeviceGroupServices.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create device group Services")
		return err
	}

	err = d.Ingresses.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Ingresses")
		return err
	}

	err = d.Configmaps.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Configmaps")
		return err
	}

	err = d.Secrets.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Secrets")
		return err
	}

	// Build storage
	err = d.Storage.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create storage for")
		return err
	}

	err = d.LoadBalancers.Build()
	if err != nil {
		log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create load balancers for")
		return err
	}

	return nil
}

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

	// Deploy Secrets
	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Secrets")
	err := d.Secrets.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Secrets, aborting")
		return err
	}

	// Deploy Configmaps
	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Configmaps")
	err = d.Configmaps.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Configmaps, aborting")
		return err
	}

	// Deploy Storage
	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Storage")
	err = d.Storage.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Storage, aborting")
		return err
	}

	// Deploy Deployments
	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Deployments")
	err = d.Deployments.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Deployments, aborting")
		return err
	}
	// Deploy Services
	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Services")
	err = d.Services.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Services, aborting")
		return err
	}

	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Device Group Services")
	err = d.DeviceGroupServices.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying DeviceGroup Services, aborting")
		return err
	}

	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Ingresses")
	err = d.Ingresses.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Ingresses, aborting")
		return err
	}

	log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Load Balancer")
	err = d.LoadBalancers.Deploy(controller)
	if err != nil {
		log.Error().Err(err).Msg("error deploying Load Balancers, aborting")
		return err
	}

	return nil
}

func (d DeployableKubernetesStage) Undeploy() error {
	// Deploying the Namespace should be enough
	// Deploy Namespace
	/*
	   err := d.Namespace.Undeploy()
	   if err != nil {
	       return err
	   }
	*/
	// Deploy Deployments
	err := d.Deployments.Undeploy()
	if err != nil {
		return err
	}
	// Deploy Services
	err = d.Services.Undeploy()
	if err != nil {
		return err
	}
	err = d.DeviceGroupServices.Undeploy()
	if err != nil {
		return err
	}
	err = d.Ingresses.Undeploy()
	if err != nil {
		return err
	}
	err = d.Configmaps.Undeploy()
	if err != nil {
		return err
	}
	err = d.Secrets.Undeploy()
	if err != nil {
		return err
	}

	err = d.Storage.Undeploy()
	if err != nil {
		return err
	}

	err = d.Storage.Undeploy()
	if err != nil {
		return err
	}
	return nil
}

func getServicePorts(ports []*pbApplication.Port) []apiv1.ServicePort {
	obtained := make([]apiv1.ServicePort, 0, len(ports))
	for _, p := range ports {
		obtained = append(obtained, apiv1.ServicePort{
			//Name:       p.Name,
			Port:       p.ExposedPort,
			TargetPort: intstr.IntOrString{IntVal: p.InternalPort},
			Name: fmt.Sprintf("port%d",p.ExposedPort),
		})
	}
	if len(obtained) == 0 {
		return nil
	}
	return obtained
}
