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
	"github.com/nalej/grpc-conductor-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// DeployableDeviceGroups contains the Services that will be created in order to expose
// endpoints to a set of device groups.
type DeployableDeviceGroups struct {
	// kubernetes Client
	client v12.ServiceInterface
	// Deployment metadata
	data         entities.DeploymentMetadata
	platformType grpc_installer_go.Platform
	Services     []ServiceInfo
}

func NewDeployableDeviceGroups(client *kubernetes.Clientset, data entities.DeploymentMetadata) *DeployableDeviceGroups {
	return &DeployableDeviceGroups{
		client:       client.CoreV1().Services(data.Namespace),
		data:         data,
		platformType: config.GetConfig().TargetPlatform,
		Services:     make([]ServiceInfo, 0),
	}
}

func (d *DeployableDeviceGroups) GetId() string {
	return d.data.Stage.StageId
}

func (d *DeployableDeviceGroups) GetServiceInfo() []ServiceInfo {
	return d.Services
}

// getK8sService creates a new service with different options depending on the target platform.
func (d *DeployableDeviceGroups) getK8sService(sr *grpc_conductor_go.DeviceGroupSecurityRuleInstance) *v1.Service {

	var serviceType = v1.ServiceTypeLoadBalancer
	if d.platformType == grpc_installer_go.Platform_MINIKUBE {
		serviceType = v1.ServiceTypeNodePort
	}

	log.Debug().Str("serviceType", string(serviceType)).Str("platformType", d.platformType.String()).Msg("device group service config")

	// Define the port that will be exposed
	var exposedPort = v1.ServicePort{
		Name:     "dg-port",
		Protocol: v1.ProtocolTCP,
		Port:     sr.TargetPort,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: sr.TargetPort,
		},
	}

	serviceName := fmt.Sprintf("dg-%s-%s", sr.RuleId, sr.TargetServiceInstanceId)
	if len(serviceName) > common.MaxNameLength {
		serviceName = serviceName[0:common.MaxNameLength]
	}

	return &v1.Service{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      serviceName,
			Namespace: d.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT: d.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:     d.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:      d.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:     d.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:            d.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SECURITY_RULE_ID:    sr.RuleId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:          sr.TargetServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID: sr.TargetServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_PURPOSE:     utils.NALEJ_ANNOTATION_VALUE_DEVICE_GROUP_SERVICE,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{exposedPort},
			Selector: map[string]string{
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:     d.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:      d.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:     d.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:            d.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:          sr.TargetServiceId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID: sr.TargetServiceInstanceId,
			},
			Type: serviceType,
		},
	}
}

func (d *DeployableDeviceGroups) createService(sr *grpc_conductor_go.DeviceGroupSecurityRuleInstance) ServiceInfo {
	service := d.getK8sService(sr)
	return ServiceInfo{
		ServiceId:         sr.TargetServiceId,
		ServiceInstanceId: sr.TargetServiceInstanceId,
		Service:           *service,
	}
}

func (d *DeployableDeviceGroups) Build() error {
	for _, sr := range d.data.Stage.DeviceGroupRules {
		d.Services = append(d.Services, d.createService(sr))
	}
	log.Debug().Interface("Num", len(d.Services)).Msg("Services for device groups have been build and are ready to deploy")
	return nil
}

func (d *DeployableDeviceGroups) Deploy(controller executor.DeploymentController) error {
	for _, servInfo := range d.Services {
		created, err := d.client.Create(&servInfo.Service)
		if err != nil {
			log.Error().Err(err).Msgf("error creating service %s", servInfo.Service.Name)
			return err
		}
		log.Debug().Str("uid", string(created.GetUID())).
			Str("name", servInfo.Service.Name).
			Str("serviceID", servInfo.ServiceId).Msg("add service resource to be monitored")
		//res := entities.NewMonitoredPlatformResource(string(created.GetUID()), d.Data, servInfo.ServiceId, servInfo.ServiceInstanceId,"")
		res := entities.NewMonitoredPlatformResource(created.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT], string(created.GetUID()),
			created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
		controller.AddMonitoredResource(&res)
	}
	return nil
}

func (d *DeployableDeviceGroups) Undeploy() error {
	for _, servInfo := range d.Services {
		err := d.client.Delete(servInfo.Service.Name, metaV1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
		if err != nil {
			log.Error().Err(err).Msgf("error deleting service agent %s", servInfo.Service.Name)
			return err
		}
	}
	return nil
}
