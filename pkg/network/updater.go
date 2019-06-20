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
func (knu *KubernetesNetworkUpdater) GetTargetNamespace(organizationID string, appInstanceID string) (string, bool, derrors.Error) {
	ns := knu.client.CoreV1().Namespaces()
	//labelSelector := v1.LabelSelector{MatchLabels: map[string]string{utils.NALEJ_ANNOTATION_ORGANIZATION_ID:organizationID, utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:appInstanceID}}
	// At this point, using the labels that we used to create the namespace does not retrieve the namespace. It may happen that K8s is doing
	// some translation that does not happen at label selector time.
	labelSelector := v1.LabelSelector{MatchLabels: map[string]string{"nalej-organization": organizationID, "nalej-app-instance-id": appInstanceID}}
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
	// TODO Check if we are affected by redeploys with namespaces that are being terminated.
	if len(list.Items) != 1 {
		return "", false, derrors.NewInternalError("multiple namespaces found for the same application instance").WithParams(list.Items)
	}
	name := list.Items[0].Name
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
		orgID, existsOrgID := pod.Labels["nalej-organization"]
		appInstID, existsAppInstID := pod.Labels["nalej-app-instance-id"]
		servGroupID, existsServGroupID := pod.Labels["nalej-service-group-id"]
		servID, existServID := pod.Labels["nalej-service-id"]
		if existsOrgID && existsAppInstID && existsServGroupID && existServID && orgID == organizationID && appInstID == appInstanceID && servGroupID == serviceGroupID && servID == serviceID{
			for _, container := range pod.Spec.Containers {
				// log.Debug().Str("name", container.Name).Interface("container", container).Msg("Container info")
				if hasEnvVar(container, utils.NALEJ_ENV_IS_PROXY) {
					log.Debug().Str("name", container.Name).Str("podIP", pod.Status.PodIP).Msg("ZT sidecar container detected")
					toAdd := NewTargetPod(pod.Name, container.Name, false, pod.Status.PodIP)
					targetPods = append(targetPods, *toAdd)
				}
			}
		}

		/*
		// TODO Check if we are going to send rules to the proxy. For now, only outbound rules are configured.
		// Check if we are dealing with a proxy
		value, existAgent := pod.Labels["agent"]
		if existAgent && value == "zt-agent" {
			log.Debug().Str("name", pod.Name).Str("podIP", pod.Status.PodIP).Msg("ZT service proxy detected")
			toAdd := NewTargetPod(pod.Name, pod.Name, false, pod.Status.PodIP)
			targetPods = append(targetPods, *toAdd)
		}
		*/
	}
	return targetPods, nil
}

func hasEnvVar(container coreV1.Container, name string) bool {
	for _, containerVar := range container.Env {
		//log.Debug().Str("container", container.Name).Str("name", containerVar.Name).Str("value", containerVar.Value).Msg("checking variables")
		if containerVar.Name == name {
			return true
		}
	}
	return false
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
	log.Debug().Msg("all pods have been updated with the new route")
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
