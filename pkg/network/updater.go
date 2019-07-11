package network

import (
	"context"
	"fmt"
	"github.com/nalej/deployment-manager/pkg/utils"
	"time"

	"github.com/nalej/derrors"
	"github.com/nalej/grpc-zt-nalej-go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const DefaultSidecarTimeout = time.Second * 10

// NetworkUpdater interface with the operations to manage the update of network information on application pods.
type NetworkUpdater interface {
	// GetTargetNamespace obtains the namespace where the application runs.
	GetTargetNamespace(organizationID string, appInstanceID string) (string, bool, derrors.Error)
	// GetPodsForApp returns the list of pods to be updated for a given app
	GetPodsForApp(namespace string, organizationID string, appInstanceID string, serviceGroupID string, serviceID string) ([]TargetPod, derrors.Error)
	// UpdatePodsRoute updates a set of pods with a given route.
	UpdatePodsRoute(targetPods []TargetPod, route *grpc_zt_nalej_go.Route) derrors.Error
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
		if namespace.Status.Phase != coreV1.NamespaceTerminating{
			// if this is not terminating we assume it is correct
			targetNamespace = &namespace
		}
	}

	if targetNamespace == nil  {
		return "", false, derrors.NewInternalError("running namespaces not found").WithParams(list.Items)
	}

	name := targetNamespace.Name
	log.Debug().Str("name", name).Msg("Target namespace has been identified")
	return name, true, nil
}

// GetPodsForApp returns the list of pods to be updated for a given app
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
			appInstID == appInstanceID && servGroupID == serviceGroupID && servID == serviceID{
			for _, container := range pod.Spec.Containers {

				if pod.Status.Phase == coreV1.PodFailed {
					// failed pods are ignored
					log.Debug().Str("namespace",namespace).Str("podName", pod.Name).Msg("ignore pod in failed status")
					continue
				}

				// Any outbound pod must have the NALEJ_ENV_IS_PROXY flag with false value. Otherwise, they indicate a
				// zt-proxy.
				if hasVar, valueVar := hasEnvVar(container, utils.NALEJ_ENV_IS_PROXY); hasVar {
					log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).
						Str("is a proxy?", valueVar).Msg("ZT sidecar container detected")
					// the value must be a boolean
					isProxy := bool(hasVar)
					toAdd := NewTargetPod(pod.Name, container.Name, isProxy, pod.Status.PodIP)
					targetPods = append(targetPods, *toAdd)
				}
			}
		}
	}
	return targetPods, nil
}

func hasEnvVar(container coreV1.Container, name string) (bool,string) {
	for _, containerVar := range container.Env {
		if containerVar.Name == name {
			return true, containerVar.Value
		}
	}
	return false,""
}

// UpdatePodsRoute updates a set of pods with a given route.
func (knu *KubernetesNetworkUpdater) UpdatePodsRoute(targetPods []TargetPod, route *grpc_zt_nalej_go.Route) derrors.Error {
	log.Debug().Interface("route", route).Int("num pods", len(targetPods)).Msg("updating pod routes")
	for _, target := range targetPods {
		log.Debug().Str("pod", target.PodName).Str("container", target.ContainerName).Msg("Updating pod")
		client, ctx, cancel, err := knu.getSidecarClient(target.PodIP)
		if err != nil {
			return err
		}
		defer cancel()
		_, rerr := client.SetRoute(ctx, route)
		if rerr != nil {
			return derrors.AsError(rerr, "cannot update route on pod")
		}
	}
	return nil
}

func (knu *KubernetesNetworkUpdater) getSidecarClient(targetIP string) (grpc_zt_nalej_go.SidecarClient, context.Context, context.CancelFunc, derrors.Error) {
	address := fmt.Sprintf("%s:1000", targetIP)
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, nil, nil, derrors.AsError(err, "cannot create connection with unified logging coordinator")
	}
	client := grpc_zt_nalej_go.NewSidecarClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSidecarTimeout)
	return client, ctx, cancel, nil
}
