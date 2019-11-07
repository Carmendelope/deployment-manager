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

// Controller implements event.DispatchFuncs to get events callbacks
// and executor.DeploymentController to add and update its MonitoredInstances.
// Created this with a MonitoredInstances, use it to create an
// events.Dispatcher which is added to the Kubernetes events provider.

import (
	"fmt"

	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/internal/structures/monitor"
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/kubernetes/events"
	"github.com/nalej/deployment-manager/pkg/utils"

	"github.com/rs/zerolog/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// The kubernetes controllers has a set of queues monitoring k8s related operations.
type KubernetesController struct {
	// Pending checks to run
	monitoredInstances monitor.MonitoredInstances
}

// Create a new kubernetes controller that handles resource events and
// updates the MonitoredInstances.
func NewKubernetesController(monitoredInstances monitor.MonitoredInstances) *KubernetesController {
	return &KubernetesController{
		monitoredInstances: monitoredInstances,
	}
}

// No-op; these dispatcher functions don't use the object store
func (c *KubernetesController) SetStore(kind schema.GroupVersionKind, store cache.Store) error {
	return nil
}

func (c *KubernetesController) SupportedKinds() events.KindList {
	return events.KindList{
		DeploymentKind,
		ServiceKind,
		IngressKind,
		// TODO decide how to proceed with namespaces control
	}
}

// Add a resource to be monitored indicating its id on the target platform (uid) and the stage identifier.
func (c *KubernetesController) AddMonitoredResource(resource *entities.MonitoredPlatformResource) {
	c.monitoredInstances.AddPendingResource(resource)
}

// Set the status of a native resource
func (c *KubernetesController) SetResourceStatus(fragmentId string, serviceID string, uid string,
	status entities.NalejServiceStatus, info string, endpoints []entities.EndpointInstance) error {
	return c.monitoredInstances.SetResourceStatus(fragmentId, serviceID, uid, status, info, endpoints)
}

// Event callback handlers
func (c *KubernetesController) OnDeployment(oldObj, obj interface{}, action events.EventType) error {
	dep := obj.(*appsv1.Deployment)
	log.Debug().Str("name", dep.GetName()).Str("status", dep.Status.String()).Msg("deployment")

	if action == events.EventDelete {
		log.Debug().Str("name", dep.GetName()).Msg("deployment deleted")
		return nil
	}

	// This deployment is monitored, and all its replicas are available
	// if there are enough replicas, we assume this is working
	if dep.Status.UnavailableReplicas == 0 && dep.Status.AvailableReplicas > 0 {
		return c.monitoredInstances.SetResourceStatus(dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],
			dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], string(dep.GetUID()),
			entities.NALEJ_SERVICE_RUNNING, "", []entities.EndpointInstance{})
	}

	foundStatus := entities.KubernetesDeploymentStatusTranslation(dep.Status)
	// Generate an information string if possible
	info := ""
	if len(dep.Status.Conditions) != 0 {
		for _, condition := range dep.Status.Conditions {
			info = fmt.Sprintf("%s %s", info, condition)
		}
	}
	log.Debug().Str(utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT, dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT]).
		Str(utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID, dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]).
		Str("uid", string(dep.GetUID())).Interface("status", foundStatus).
		Msg("set deployment status")
	return c.monitoredInstances.SetResourceStatus(dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],
		dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], string(dep.GetUID()), foundStatus, info, []entities.EndpointInstance{})
}

func (c *KubernetesController) OnService(oldObj, obj interface{}, action events.EventType) error {
	// TODO determine what do we expect from a service to be deployed
	dep := obj.(*corev1.Service)
	log.Debug().Str("name", dep.GetName()).Str("status", dep.Status.String()).Msg("service")

	if action == events.EventDelete {
		log.Debug().Str("name", dep.GetName()).Msg("service deleted")
		return nil
	}

	endpoints := make([]entities.EndpointInstance, 0)

	purpose, found := dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_PURPOSE]
	if found && purpose == utils.NALEJ_ANNOTATION_VALUE_DEVICE_GROUP_SERVICE {
		log.Debug().Interface("analyzing", dep).Msg("Checking service for device group ingestion")

		if dep.Spec.Type == corev1.ServiceTypeLoadBalancer {
			log.Debug().Msg("Load balancer detected")
			if dep.Status.LoadBalancer.Ingress == nil || len(dep.Status.LoadBalancer.Ingress) == 0 {
				log.Debug().Interface("loadbalancer", dep.Status).Msg("Load balancer is not ready, skip")
				return nil
			}

			for _, ip := range dep.Status.LoadBalancer.Ingress {
				for _, port := range dep.Spec.Ports {
					ep := entities.EndpointInstance{
						EndpointInstanceId: string(dep.UID),
						EndpointType:       entities.ENDPOINT_TYPE_INGESTION,
						FQDN:               ip.IP,
						Port:               port.Port,
					}
					log.Debug().Interface("endpoint", ep).Msg("Load balancer is ready")
					endpoints = append(endpoints, ep)
				}
			}
		} else if dep.Spec.Type == corev1.ServiceTypeNodePort {
			log.Debug().Msg("Node port detected")
			for _, port := range dep.Spec.Ports {
				ep := entities.EndpointInstance{
					EndpointInstanceId: string(dep.UID),
					EndpointType:       entities.ENDPOINT_TYPE_INGESTION,
					FQDN:               config.GetConfig().ClusterPublicHostname,
					Port:               port.NodePort,
				}
				log.Debug().Interface("endpoint", ep).Msg("Node port is ready")
				endpoints = append(endpoints, ep)
			}
		}
	} else if found && purpose == utils.NALEJ_ANNOTATION_VALUE_LOAD_BALANCER_SERVICE {
		log.Debug().Interface("analyzing", dep).Msg("Checking service for load balancer")
		if dep.Spec.Type == corev1.ServiceTypeLoadBalancer {
			log.Debug().Msg("Load balancer detected")
			if dep.Status.LoadBalancer.Ingress == nil || len(dep.Status.LoadBalancer.Ingress) == 0 {
				log.Debug().Interface("loadbalancer", dep.Status).Msg("Load balancer is not ready, skip")
				return nil
			}

			for _, ip := range dep.Status.LoadBalancer.Ingress {
				for _, port := range dep.Spec.Ports {
					ep := entities.EndpointInstance{
						EndpointInstanceId: string(dep.UID),
						EndpointType:       entities.ENDPOINT_TYPE_INGESTION,
						FQDN:               ip.IP,
						Port:               port.Port,
					}
					log.Debug().Interface("endpoint", ep).Msg("Load balancer is ready")
					endpoints = append(endpoints, ep)
				}
			}
		}
	}

	log.Debug().Str(utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT, dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT]).
		Str(utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID, dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]).
		Str("uid", string(dep.GetUID())).Interface("status", entities.NALEJ_SERVICE_RUNNING).
		Msg("set service new status to ready")
	return c.monitoredInstances.SetResourceStatus(dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],
		dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], string(dep.GetUID()), entities.NALEJ_SERVICE_RUNNING, "",
		endpoints)
}

func (c *KubernetesController) OnIngress(oldObj, obj interface{}, action events.EventType) error {
	dep := obj.(*extensionsv1beta1.Ingress)
	log.Debug().Str("name", dep.GetName()).Str("status", dep.Status.String()).Msg("ingress")

	if action == events.EventDelete {
		log.Debug().Str("name", dep.GetName()).Msg("ingress deleted")
		return nil
	}

	//  It considers the ingress to be ready when all the entries have ip and hostname
	ready := true
	for _, ing := range dep.Status.LoadBalancer.Ingress {
		if ing.Hostname == "" || ing.IP == "" {
			ready = false
			break
		}
	}

	if ready && len(dep.Spec.Rules) > 0 {
		port := int32(0)
		if len(dep.Spec.Rules) > 0 && len(dep.Spec.Rules[0].IngressRuleValue.HTTP.Paths) > 0 {
			port = dep.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal
		}

		// Take the local cluster hostname.
		hostname := dep.Spec.Rules[0].Host

		log.Debug().Str(utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT, dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT]).
			Str(utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID, dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]).
			Str("uid", string(dep.GetUID())).Interface("status", entities.NALEJ_SERVICE_RUNNING).
			Msg("set ingress new status to ready")

		return c.monitoredInstances.SetResourceStatus(dep.Labels[utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT],
			dep.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], string(dep.GetUID()), entities.NALEJ_SERVICE_RUNNING,
			"", []entities.EndpointInstance{entities.EndpointInstance{
				FQDN:               hostname,
				EndpointInstanceId: string(dep.UID),
				EndpointType:       entities.ENDPOINT_TYPE_WEB,
				Port:               port,
			}})
	}

	return nil
}
