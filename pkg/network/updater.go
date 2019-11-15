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

package network

import (
	"context"
	"fmt"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-zt-nalej-go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"time"
)

const (
	DefaultSidecarTimeout = time.Second * 10
	ZtRedirectorPort      = 1576
	UpdateRetries         = 3
	JoinRetries           = 3
	RetrySleep            = 2 * time.Second
)

// NetworkUpdater interface with the operations to manage the update of network information on application pods.
type NetworkUpdater interface {
	// GetTargetNamespace obtains the namespace where the application runs.
	GetTargetNamespace(organizationID string, appInstanceID string) (string, bool, derrors.Error)
	// GetPodsForApp returns the list of pods to be updated for a given app
	GetPodsForApp(namespace string, organizationID string, appInstanceID string, serviceGroupID string, serviceID string) ([]TargetPod, derrors.Error)
	// GetPodsForApp returns the list of pods to be updated for a given app (including zt-proxy)
	GetAllPodsForApp(namespace string, organizationID string, appInstanceID string, serviceID string) ([]TargetPod, derrors.Error)
	// UpdatePodsRoute updates a set of pods with a given route.
	UpdatePodsRoute(targetPods []TargetPod, route *grpc_zt_nalej_go.Route) derrors.Error
	// SendJoinZTConnection send a join message to the pods
	SendJoinZTConnection(targetPods []TargetPod, networkId string, isInbound bool) derrors.Error
	SendLeaveZTConnection(targetPods []TargetPod, networkId string, isInbound bool) derrors.Error
}

// TargetPod representing a pod to be updated.
type TargetPod struct {
	PodName       string
	ContainerName string
	OutboundProxy bool
	PodIP         string
}

// NewTargetPod creates a new TargetPod
func NewTargetPod(pod string, container string, outboundProxy bool, ip string) *TargetPod {
	return &TargetPod{pod, container, outboundProxy, ip}
}

type KubernetesNetworkUpdater struct {
	// kubernetes Client
	client *kubernetes.Clientset
}

func NewKubernetesNetworkUpdater(client *kubernetes.Clientset) NetworkUpdater {
	return &KubernetesNetworkUpdater{client}
}

// GetTargetNamespace obtains the namespace where the application runs.
// params:
//  organizationID
//  appInstanceID
// return:
//  the target namespace if found
//  a boolean whether we found it or not
//  any found error
func (knu *KubernetesNetworkUpdater) GetTargetNamespace(organizationID string, appInstanceID string) (string, bool, derrors.Error) {
	ns := knu.client.CoreV1().Namespaces()

	// At this point, using the labels that we used to create the namespace does not retrieve the namespace. It may happen that K8s is doing
	// some translation that does not happen at label selector time.
	labelSelector := v1.LabelSelector{MatchLabels: map[string]string{utils.NALEJ_ANNOTATION_ORGANIZATION_ID: organizationID, utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: appInstanceID}}
	opts := v1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	log.Debug().Str("organizationID", organizationID).Str("appInstanceID", appInstanceID).Interface("opts", opts).Msg("Getting target namespace")
	list, err := ns.List(opts)
	if err != nil {
		return "", false, derrors.AsError(err, "cannot list namespaces")
	}
	if len(list.Items) == 0 {
		log.Debug().Msg("no namespaces found")
		return "", false, nil
	}

	// Check that it exists at least one single namespace running
	var targetNamespace *coreV1.Namespace = nil
	for _, namespace := range list.Items {
		if namespace.Status.Phase != coreV1.NamespaceTerminating {
			// if this is not terminating we assume it is correct
			targetNamespace = &namespace
		}
	}

	if targetNamespace == nil {
		return "", false, derrors.NewInternalError("running namespaces not found").WithParams(list.Items)
	}

	name := targetNamespace.Name
	log.Debug().Str("name", name).Msg("Target namespace has been identified")
	return name, true, nil
}

// GetPodsForApp returns the list of pods to be updated for a given app. No proxies are included.
func (knu *KubernetesNetworkUpdater) GetPodsForApp(namespace string, organizationID string, appInstanceID string, serviceGroupID string, serviceID string) ([]TargetPod, derrors.Error) {
	podClient := knu.client.CoreV1().Pods(namespace)
	opts := v1.ListOptions{}
	list, err := podClient.List(opts)
	if err != nil {
		return nil, derrors.AsError(err, "cannot list pods inside a namespace")
	}
	targetPods := make([]TargetPod, 0)
	for _, pod := range list.Items {
		// Check if we are inspecting a pod of the application. While we expect only for those pods to be present, we may be also running tests with manually
		// created pods, or we may run other pods in the future.
		orgID, existsOrgID := pod.Labels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID]
		appInstID, existsAppInstID := pod.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID]
		servGroupID, existsServGroupID := pod.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID]
		servID, existServID := pod.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID]

		if existsOrgID && existsAppInstID && existsServGroupID && existServID && orgID == organizationID &&
			appInstID == appInstanceID && servGroupID == serviceGroupID && servID == serviceID {
			for _, container := range pod.Spec.Containers {

				if pod.Status.Phase == coreV1.PodFailed {
					// failed pods are ignored
					log.Debug().Str("namespace", namespace).Str("podName", pod.Name).Msg("ignore pod in failed status")
					continue
				}

				// Any outbound pod must have the NALEJ_ENV_IS_PROXY flag with false value. Otherwise, they indicate a
				// zt-proxy.
				if hasVar, valueVar := hasEnvVar(container, utils.NALEJ_ENV_IS_PROXY); hasVar {
					log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
						Str("is a proxy?", valueVar).Msg("ZT sidecar container detected")
					// the value must be a boolean
					isProxy, err := strconv.ParseBool(valueVar)
					if err != nil {
						log.Error().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
							Str(utils.NALEJ_ANNOTATION_IS_PROXY, valueVar).
							Msg("the env variable should be boolean")
						continue
					}
					if !isProxy {
						log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
							Str("is a proxy?", valueVar).Msg("ZT sidecar added to the candidate list")
						toAdd := NewTargetPod(pod.Name, container.Name, isProxy, pod.Status.PodIP)
						targetPods = append(targetPods, *toAdd)
					}
				}
			}
		}
	}
	return targetPods, nil
}

func (knu *KubernetesNetworkUpdater) GetAllPodsForApp(namespace string, organizationID string, appInstanceID string, serviceID string) ([]TargetPod, derrors.Error) {
	podClient := knu.client.CoreV1().Pods(namespace)
	opts := v1.ListOptions{}
	list, err := podClient.List(opts)
	if err != nil {
		return nil, derrors.AsError(err, "cannot list pods inside a namespace")
	}

	targetPods := make([]TargetPod, 0)
	for _, pod := range list.Items {
		// Check if we are inspecting a pod of the application. While we expect only for those pods to be present, we may be also running tests with manually
		// created pods, or we may run other pods in the future.
		orgID, existsOrgID := pod.Labels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID]
		appInstID, existsAppInstID := pod.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID]
		servID, existServID := pod.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID]

		if existsOrgID && existsAppInstID && existServID && orgID == organizationID &&
			appInstID == appInstanceID && servID == serviceID {
			for _, container := range pod.Spec.Containers {

				if pod.Status.Phase == coreV1.PodFailed {
					// failed pods are ignored
					log.Debug().Str("namespace", namespace).Str("podName", pod.Name).Msg("ignore pod in failed status")
					continue
				}

				// Any outbound pod must have the NALEJ_ENV_IS_PROXY flag with false value. Otherwise, they indicate a
				// zt-proxy.
				hasVar, valueVar := hasEnvVar(container, utils.NALEJ_ENV_IS_PROXY)
				if hasVar {
					log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
						Str("is a proxy?", valueVar).Msg("ZT sidecar container detected")
					// the value must be a boolean
					isProxy, err := strconv.ParseBool(valueVar)
					if err != nil {
						log.Error().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
							Str(utils.NALEJ_ANNOTATION_IS_PROXY, valueVar).
							Msg("the env variable should be boolean")
						continue
					}
					log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
						Str("is a proxy?", valueVar).Msg("ZT sidecar added to the candidate list")
					toAdd := NewTargetPod(pod.Name, container.Name, isProxy, pod.Status.PodIP)
					targetPods = append(targetPods, *toAdd)

				} else {
					log.Debug().Interface("container", container).Msg("has no env var")
				}
			}
		}
	}
	return targetPods, nil
}

func hasEnvVar(container coreV1.Container, name string) (bool, string) {
	for _, containerVar := range container.Env {
		if containerVar.Name == name {
			return true, containerVar.Value
		}
	}
	return false, ""
}

// UpdatePodsRoute updates a set of pods with a given route.
func (knu *KubernetesNetworkUpdater) UpdatePodsRoute(targetPods []TargetPod, route *grpc_zt_nalej_go.Route) derrors.Error {
	log.Debug().Interface("route", route).Int("num pods", len(targetPods)).Msg("updating pod routes")
	for _, target := range targetPods {
		log.Debug().Str("pod", target.PodName).Str("podIp", target.PodIP).
			Str("container", target.ContainerName).Msg("Updating pod")
		client, ctx, cancel, err := knu.getSidecarClient(target.PodIP)
		if cancel != nil {
			defer cancel()
		}
		if err != nil {
			log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
				Msg("error when recovering a client to interact with sidecar pod")
			return err
		}
		done := false
		var rerr error = nil
		for attempts := 0; attempts < UpdateRetries; attempts++ {
			_, rerr = client.SetRoute(ctx, route)
			if rerr != nil {
				log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
					Msgf("cannot update route on pod on attempt %d out of %d", attempts+1, UpdateRetries)
				time.Sleep(RetrySleep)
			} else {
				log.Debug().Str("pod", target.PodName).Str("container", target.ContainerName).
					Msg("pod route update was done")
				done = true
				break
			}
		}
		if !done {
			return derrors.AsError(rerr, "cannot update route on pod")
		}

	}
	return nil
}

func (knu *KubernetesNetworkUpdater) JoinMustBeSent(pod TargetPod, isInbound bool) bool {
	//
	if (isInbound && pod.OutboundProxy) ||
		(!isInbound && !pod.OutboundProxy) {
		return true
	}
	return false
}

// SendJoinZTConnection send a message to the pods to join into a new ZT Network.
// If isInbound -> the message is only sent to the ZT-proxy pod
// If is not inbound (is an outbound) -> the message is sent into pods that are not ZT-proxy
func (knu *KubernetesNetworkUpdater) SendJoinZTConnection(targetPods []TargetPod, networkId string, isInbound bool) derrors.Error {
	log.Debug().Interface("networkId", networkId).Int("num pods", len(targetPods)).Msg("join network")

	for _, target := range targetPods {
		send := knu.JoinMustBeSent(target, isInbound)

		log.Debug().Str("pod", target.PodName).Str("podIp", target.PodIP).Bool("IsInbound", isInbound).Bool("send the join?", send).
			Str("container", target.ContainerName).Str("networkId", networkId).Msg("Join into a zt-network")

		if send {

			client, ctx, cancel, err := knu.getApplicationNetworkClient(target.PodIP)
			if cancel != nil {
				defer cancel()
			}
			if err != nil {
				log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
					Msg("error when recovering a client to interact with sidecar pod")
				return err
			}
			done := false
			var rerr error = nil
			for attempts := 0; attempts < JoinRetries; attempts++ {
				_, rerr = client.JoinZTNetwork(ctx, &grpc_zt_nalej_go.JoinZTNetworkRequest{
					NetworkId:           networkId,
					IsInboundConnection: isInbound,
				})
				if rerr != nil {
					log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
						Msgf("cannot join into zt-network on pod on attempt %d out of %d", attempts+1, UpdateRetries)
					time.Sleep(RetrySleep)
				} else {
					log.Debug().Str("pod", target.PodName).Str("container", target.ContainerName).
						Msg("join into zt-network was done")
					done = true
					break
				}
			}
			if !done {
				return derrors.AsError(rerr, "cannot join into zt-network on pod")
			}
		}

	}
	return nil
}

func (knu *KubernetesNetworkUpdater) SendLeaveZTConnection(targetPods []TargetPod, networkId string, isInbound bool) derrors.Error {
	log.Debug().Interface("networkId", networkId).Int("num pods", len(targetPods)).Msg("leave zt-network")

	for _, target := range targetPods {
		send := knu.JoinMustBeSent(target, isInbound)

		log.Debug().Str("pod", target.PodName).Str("podIp", target.PodIP).Bool("IsInbound", isInbound).Bool("send the leave?", send).
			Str("container", target.ContainerName).Str("networkId", networkId).Msg("Leave a zt-network")

		if send {

			client, ctx, cancel, err := knu.getApplicationNetworkClient(target.PodIP)
			if cancel != nil {
				defer cancel()
			}
			if err != nil {
				log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
					Msg("error when recovering a client to interact with sidecar pod")
				return err
			}
			done := false
			var rerr error = nil
			for attempts := 0; attempts < JoinRetries; attempts++ {
				_, rerr = client.LeaveZTNetwork(ctx, &grpc_zt_nalej_go.ZTNetworkId{
					NetworkId: networkId,
				})
				if rerr != nil {
					log.Error().Str("pod", target.PodName).Str("container", target.ContainerName).
						Msgf("cannot send the message to leave zt-network on pod on attempt %d out of %d", attempts+1, UpdateRetries)
					time.Sleep(RetrySleep)
				} else {
					log.Debug().Str("pod", target.PodName).Str("container", target.ContainerName).
						Msg("leave zt-network Success")
					done = true
					break
				}
			}
			if !done {
				return derrors.AsError(rerr, "cannot leave zt-network on pod")
			}
		}

	}
	return nil
}

func (knu *KubernetesNetworkUpdater) getSidecarClient(targetIP string) (grpc_zt_nalej_go.SidecarClient, context.Context, context.CancelFunc, derrors.Error) {
	address := fmt.Sprintf("%s:%d", targetIP, ZtRedirectorPort)
	log.Debug().Msgf("get sidecar client for %s",address)
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, nil, nil, derrors.AsError(err, "cannot create connection with unified logging coordinator")
	}
	client := grpc_zt_nalej_go.NewSidecarClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSidecarTimeout)
	return client, ctx, cancel, nil
}

// ApplicationNetworkClient
func (knu *KubernetesNetworkUpdater) getApplicationNetworkClient(targetIP string) (grpc_zt_nalej_go.ApplicationNetworkClient, context.Context, context.CancelFunc, derrors.Error) {
	address := fmt.Sprintf("%s:%d", targetIP, ZtRedirectorPort)
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, nil, nil, derrors.AsError(err, "cannot create connection with unified logging coordinator")
	}
	client := grpc_zt_nalej_go.NewApplicationNetworkClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSidecarTimeout)
	return client, ctx, cancel, nil
}
