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
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/rs/zerolog/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
)

//--------------------
// Deployable Services
//--------------------

type ServiceInfo struct {
	ServiceId         string
	ServiceInstanceId string
	Service           apiv1.Service
}

type DeployableServices struct {
	// kubernetes Client
	Client v12.ServiceInterface
	// Deployment metadata
	Data entities.DeploymentMetadata
	// [[ServiceId, ServiceInstanceId, Service]...]
	Services []ServiceInfo
	// Network decorator
	networkDecorator executor.NetworkDecorator
}

func NewDeployableService(client *kubernetes.Clientset, data entities.DeploymentMetadata,
	decorator executor.NetworkDecorator) *DeployableServices {

	return &DeployableServices{
		Client:   client.CoreV1().Services(data.Namespace),
		Data:     data,
		Services: make([]ServiceInfo, 0),
		networkDecorator: decorator,
	}
}

func (d *DeployableServices) GetId() string {
	return d.Data.Stage.StageId
}

func (s *DeployableServices) Build() error {
	for serviceIndex, service := range s.Data.Stage.Services {

		// Check if the service already exists
		_, err := s.Client.Get(common.FormatName(service.ServiceName),metav1.GetOptions{})
		if !errors.IsNotFound(err)  {
			log.Debug().Str("serviceName",service.ServiceName).Msg("the service already exists, no need to be be created")
			continue
		}

		log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.Data.Stage.Services))


		extendedLabels := make(map[string]string, 0)
		extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = s.Data.OrganizationId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = s.Data.AppDescriptorId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = s.Data.AppInstanceId
		/*
		extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = s.Data.FragmentId
		extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = s.Data.OrganizationId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = s.Data.AppDescriptorId
		extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = s.Data.AppInstanceId
		extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = s.Data.Stage.StageId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
		extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
		*/
		extendedLabels[utils.NALEJ_ANNOTATION_IS_PROXY] = "false"

		// Labels for the selector
		selectorLabels := map[string]string{
			utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: s.Data.AppInstanceId,
			utils.NALEJ_ANNOTATION_ORGANIZATION_ID: service.OrganizationId,
			utils.NALEJ_ANNOTATION_SERVICE_NAME: common.FormatName(service.ServiceName),
		}

		ports := getServicePorts(service.ExposedPorts)
		if ports != nil {
			k8sService := apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: s.Data.Namespace,
					Name:      common.FormatName(service.ServiceName),
					Labels:    extendedLabels,
				},
				Spec: apiv1.ServiceSpec{
					ExternalName: common.FormatName(service.ServiceName),
					Ports:        ports,
					//Type:         apiv1.ServiceTypeNodePort,
					Type:         apiv1.ServiceTypeClusterIP,
					Selector:     selectorLabels,
				},
			}
			log.Debug().Str("serviceId", service.ServiceId).Str("serviceInstanceId", service.ServiceInstanceId).
				Interface("apiv1.Service", k8sService).Msg("generated k8s service")
			s.Services = append(s.Services, ServiceInfo{service.ServiceId, service.ServiceInstanceId, k8sService})

		} else {
			log.Debug().Msgf("No k8s service is generated for %s", service.ServiceId)
		}
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := s.networkDecorator.Build(s)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error building network components")
		return errNetDecorator
	}

	return nil
}

func (s *DeployableServices) Deploy(controller executor.DeploymentController) error {
	log.Debug().Int("numberServicesToDeploy", len(s.Services)).Msg("deploy deployableServices")

	for _, servInfo := range s.Services {

		// if the service is already there skip
		_, err := s.Client.Get(servInfo.Service.Name,metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			log.Debug().Str("serviceName", servInfo.Service.Name).Msg("service already deployed. Skip it")
			continue
		}

		created, err := s.Client.Create(&servInfo.Service)
		if err != nil {
			log.Error().Err(err).Msgf("error creating service %s", servInfo.Service.Name)
			return err
		}
		log.Debug().Str("serviceName",servInfo.Service.Name).Str("uid", string(created.GetUID())).
			Str("appInstanceID", s.Data.AppInstanceId).
			Str("serviceID", servInfo.ServiceId).Msg("add service resource to be monitored")

		res := entities.NewMonitoredPlatformResource(created.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT], string(created.GetUID()),
			created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
		controller.AddMonitoredResource(&res)
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := s.networkDecorator.Deploy(s)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error deploying network components")
		return errNetDecorator
	}
	return nil
}


func (s *DeployableServices) Undeploy() error {
	for _, servInfo := range s.Services {
		err := s.Client.Delete(common.FormatName(servInfo.Service.Name), metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
		if err != nil {
			log.Error().Err(err).Msgf("error deleting service %s", servInfo.Service.Name)
			return err
		}
	}

	// call the network decorator and modify deployments accordingly
	errNetDecorator := s.networkDecorator.Undeploy(s)
	if errNetDecorator != nil {
		log.Error().Err(errNetDecorator).Msg("error undeploying network components")
		return errNetDecorator
	}

	return nil
}
