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
 *
 */

package zerotier

import (
	"fmt"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/kubernetes"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Name of the Docker ZT agent image
	ZtAgentImageName = "nalej/zt-agent:v0.4.0"
	// Default imagePullPolicy
	DefaultImagePullPolicy = apiv1.PullAlways
	// ZtSidecarImageName
	ZtSidecarImageName = "zt-sidecar"
	// Name of the Zt sidecar port
	ZtSidecarPortName = "ztrouteport"
	// Identifier for the ZT network
	ZtNetworkId = "NALEJ_ZT_NETWORK_ID"
)

type ZerotierDecorator struct {
	// Empty
}

func NewZerotierDecorator() executor.NetworkDecorator {
	return &ZerotierDecorator{}
}

func (d *ZerotierDecorator) Build(aux executor.Deployable, args ...interface{}) derrors.Error {
	switch target := aux.(type) {
	// Process a deployment
	case *kubernetes.DeployableDeployments:
		// We expect a service to be sent as first argument
		errSidecars := d.createSidecars(target)
		if errSidecars != nil {
			return errSidecars
		}
		errInbounds := d.createInbounds(target)
		if errInbounds != nil {
			return errInbounds
		}
	default:
		// nothing to do
		return nil
	}
	return nil
}

func (d *ZerotierDecorator) Deploy(aux executor.Deployable, args ...interface{}) derrors.Error {
	// Nothing to do
	return nil
}

func (d *ZerotierDecorator) Undeploy(aux executor.Deployable, args ...interface{}) derrors.Error {
	// Nothing to do
	return nil
}

// Create the ZT sidecars that give access to the ZT network. Modify the generated deployments to include a sidecar
// with all the necessary content to deploy a ZT sidecar. This new entries are automatically added to the deployable.
// params:
//   dep the object with the deployable information
//   service the original service that is going to be translated into a set of k8s deployments
// returns:
//   error if any
func (d *ZerotierDecorator) createSidecars(dep *kubernetes.DeployableDeployments) derrors.Error {

	// value for privileged user
	user0 := int64(0)
	privilegedUser := &user0

	// extend created deployments with the ZT additional workers
	// Every deployment must have a ZT sidecar and an additional proxy if services are enabled.

	for index, service := range dep.Data.Stage.Services {

		// extend variables to indicate that this is not an inbound
		containerVars := generateContainerVars(dep, service, false)
		// the deployment to be extended is the one corresponding to this service
		toBeExtended := dep.Deployments[index]

		ztContainer := apiv1.Container{
			Name:  ZtSidecarImageName,
			Image: ZtAgentImageName,
			Args: []string{
				"run",
			},
			// Set the no proxy variable
			Env: containerVars,
			LivenessProbe: &apiv1.Probe{
				InitialDelaySeconds: 20,
				PeriodSeconds:       60,
				TimeoutSeconds:      20,
				Handler: apiv1.Handler{
					Exec: &apiv1.ExecAction{
						Command: []string{
							"./nalej/zt-agent",
							"check",
							"--appInstanceId", dep.Data.AppInstanceId,
							"--appName", dep.Data.AppName,
							"--serviceName", service.ServiceName,
							"--deploymentId", dep.Data.DeploymentId,
							"--fragmentId", dep.Data.Stage.FragmentId,
							"--managerAddr", config.GetConfig().DeploymentMgrAddress,
							"--organizationId", dep.Data.OrganizationId,
							"--organizationName", dep.Data.OrganizationName,
							"--networkId", dep.Data.ZtNetworkId,
							"--serviceGroupInstanceId", service.ServiceGroupInstanceId,
							"--serviceAppInstanceId", service.ServiceInstanceId,
						},
					},
				},
			},

			// The proxy exposes the same ports of the deployment
			Ports:           getContainerPorts(service.ExposedPorts),
			ImagePullPolicy: DefaultImagePullPolicy,
			SecurityContext: &apiv1.SecurityContext{
				RunAsUser:  privilegedUser,
				Privileged: common.BoolPtr(true),
				Capabilities: &apiv1.Capabilities{
					Add: []apiv1.Capability{
						"NET_ADMIN",
						"SYS_ADMIN",
					},
				},
			},

			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "dev-net-tun",
					ReadOnly:  true,
					MountPath: "/dev/net/tun",
				},
			},
		}

		// Extend the containers with the sidecar
		toBeExtended.Spec.Template.Spec.Containers = append(
			toBeExtended.Spec.Template.Spec.Containers, ztContainer)

		// Extend the volumes with the zt required volume
		ztVolume := apiv1.Volume{
			// zerotier sidecar volume
			Name: "dev-net-tun",
			VolumeSource: apiv1.VolumeSource{
				HostPath: &apiv1.HostPathVolumeSource{
					Path: "/dev/net/tun",
				},
			},
		}

		if len(toBeExtended.Spec.Template.Spec.Volumes) == 0 {
			toBeExtended.Spec.Template.Spec.Volumes = []apiv1.Volume{ztVolume}
		} else {
			toBeExtended.Spec.Template.Spec.Volumes = append(toBeExtended.Spec.Template.Spec.Volumes, ztVolume)
		}

		// Add DNS entries
		// Set POD DNS policies
		toBeExtended.Spec.Template.Spec.DNSPolicy = apiv1.DNSNone
		toBeExtended.Spec.Template.Spec.DNSConfig = &apiv1.PodDNSConfig{Nameservers: dep.Data.DNSHosts}

	}

	return nil
}

// Create the inbounds corresponding for inner communications in an existing service.
func (d *ZerotierDecorator) createInbounds(dep *kubernetes.DeployableDeployments) derrors.Error {

	// value for privileged user
	user0 := int64(0)
	privilegedUser := &user0

	for _, service := range dep.Data.Stage.Services {
		// If there are no exposed ports, simply return
		if len(service.ExposedPorts) == 0 {
			continue
		}

		// extend variables to indicate that this is an inbound
		extendedLabels := generateContainerLabelsInbound(dep, service)

		// extend created deployments with the ZT additional workers
		// Every deployment must have a ZT sidecar and an additional proxy if services are enabled.
		ztAgentName := fmt.Sprintf("zt-%s", common.FormatName(service.ServiceName))

		agent := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ztAgentName,
				Namespace: dep.Data.Namespace,
				Labels:    extendedLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: common.Int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: extendedLabels,
				},
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: extendedLabels,
					},
					// Every pod template is designed to use a container with the requested image
					// and a helping sidecar with a containerized zerotier that joins the network
					// after running
					Spec: apiv1.PodSpec{
						// Do not mount any service account token
						AutomountServiceAccountToken: common.BoolPtr(false),
						Containers: []apiv1.Container{
							// zero-tier sidecar
							{
								Name:  ztAgentName,
								Image: ZtAgentImageName,
								Args: []string{
									"run",
									"--debug",
								},
								Env: generateContainerVars(dep, service, true),
								LivenessProbe: &apiv1.Probe{
									InitialDelaySeconds: 20,
									PeriodSeconds:       60,
									TimeoutSeconds:      20,
									Handler: apiv1.Handler{
										Exec: &apiv1.ExecAction{
											Command: []string{
												"./nalej/zt-agent",
												"check",
												"--appInstanceId", dep.Data.AppInstanceId,
												"--appName", dep.Data.AppName,
												"--serviceName", common.FormatName(service.ServiceName),
												"--deploymentId", dep.Data.DeploymentId,
												"--fragmentId", dep.Data.Stage.FragmentId,
												"--managerAddr", config.GetConfig().DeploymentMgrAddress,
												"--organizationId", dep.Data.OrganizationId,
												"--organizationName", dep.Data.OrganizationName,
												"--networkId", dep.Data.ZtNetworkId,
												"--serviceGroupInstanceId", service.ServiceGroupInstanceId,
												"--serviceAppInstanceId", service.ServiceInstanceId,
											},
										},
									},
								},
								// The proxy exposes the same ports of the deployment + the ztport
								Ports:           getContainerPorts(service.ExposedPorts),
								ImagePullPolicy: DefaultImagePullPolicy,
								SecurityContext: &apiv1.SecurityContext{
									RunAsUser:  privilegedUser,
									Privileged: common.BoolPtr(true),
									Capabilities: &apiv1.Capabilities{
										Add: []apiv1.Capability{
											"NET_ADMIN",
											"SYS_ADMIN",
										},
									},
								},
								VolumeMounts: []apiv1.VolumeMount{
									{
										Name:      "dev-net-tun",
										ReadOnly:  true,
										MountPath: "/dev/net/tun",
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
						},
					},
				},
			},
		}

		// Add this inbound to the list of deployments
		dep.Deployments = append(dep.Deployments, &agent)
	}

	return nil
}

func generateContainerLabelsInbound(d *kubernetes.DeployableDeployments,
	service *grpc_conductor_go.ServiceInstance) map[string]string {

	extendedLabels := make(map[string]string, 0)
	if service.Labels != nil {
		// users have already defined labels for this app
		for k, v := range service.Labels {
			extendedLabels[k] = v
		}
	}

	extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.Data.FragmentId
	extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID] = d.Data.OrganizationId
	extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = d.Data.AppDescriptorId
	extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = d.Data.AppInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = d.Data.Stage.StageId
	extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_ID] = d.Data.DeploymentId
	extendedLabels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT] = d.Data.Stage.FragmentId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_IS_PROXY] = "true"

	return extendedLabels
}

// Generate labels for the Zt sidecars.
// params:
//   d deployable element with the metadata required to tag instances
//   service entry with the corresponding service metadata
//   isProxy set to true if we have to run a proxy instance
// return:
//   map of labels with their values
func generateContainerVars(d *kubernetes.DeployableDeployments,
	service *grpc_conductor_go.ServiceInstance, isProxy bool) []apiv1.EnvVar {
	return []apiv1.EnvVar{
		{
			Name: utils.NALEJ_ENV_CLUSTER_ID, Value: config.GetConfig().ClusterId,
		},
		{
			Name: utils.NALEJ_ENV_IS_PROXY, Value: fmt.Sprintf("%t", isProxy),
		},
		{
			Name: ZtNetworkId, Value: d.Data.ZtNetworkId,
		},
		{
			Name: utils.NALEJ_ENV_MANAGER_ADDR, Value: config.GetConfig().DeploymentMgrAddress,
		},
		{
			Name: utils.NALEJ_ENV_DEPLOYMENT_ID, Value: d.Data.DeploymentId,
		},
		{
			Name: utils.NALEJ_ENV_DEPLOYMENT_FRAGMENT, Value: d.Data.Stage.FragmentId,
		},
		{
			Name: utils.NALEJ_ENV_ORGANIZATION_ID, Value: d.Data.OrganizationId,
		},
		{
			Name: utils.NALEJ_ENV_ORGANIZATION_NAME, Value: d.Data.OrganizationName,
		},
		{
			Name: utils.NALEJ_ENV_APP_DESCRIPTOR, Value: d.Data.AppDescriptorId,
		},
		{
			Name: utils.NALEJ_ENV_APP_NAME, Value: d.Data.AppName,
		},
		{
			Name: utils.NALEJ_ENV_APP_INSTANCE_ID, Value: d.Data.AppInstanceId,
		},
		{
			Name: utils.NALEJ_ENV_STAGE_ID, Value: d.Data.Stage.StageId,
		},
		{
			Name: utils.NALEJ_ENV_SERVICE_NAME, Value: service.ServiceName,
		},
		{
			Name: utils.NALEJ_ENV_SERVICE_ID, Value: service.ServiceId,
		},
		{
			Name: utils.NALEJ_ENV_SERVICE_FQDN, Value: common.GetServiceFQDN(service.ServiceName, service.OrganizationId, service.AppInstanceId),
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

// Transform a Nalej list of exposed ports into a K8s api port.
//  params:
//   ports list of exposed ports
//  return:
//   list of ports into k8s api format
func getContainerPorts(ports []*grpc_application_go.Port) []apiv1.ContainerPort {
	obtained := make([]apiv1.ContainerPort, 0)
	for _, p := range ports {
		obtained = append(obtained, apiv1.ContainerPort{ContainerPort: p.ExposedPort, Name: p.Name})
	}

	obtained = append(obtained, apiv1.ContainerPort{ContainerPort: int32(config.GetConfig().ZTSidecarPort), Name: ZtSidecarPortName})

	return obtained
}
