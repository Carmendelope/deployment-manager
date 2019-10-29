/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package offline_policy

import (
	"fmt"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/nalej/deployment-manager/pkg/kubernetes"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Grace period in seconds to delete a deployable.
	DeleteGracePeriod = 10
)

type Manager struct {
	// Kubernetes client
	KubernetesClient *k8s.Clientset
}

func NewManager() *Manager {
	kClient, err  := kubernetes.GetKubernetesClient(true)
	if err != nil {
		log.Error().Err(err).Msg("error creating kubernetes client")
		return nil
	}
	return &Manager{
		KubernetesClient:kClient,
	}
}

func (m *Manager) RemoveAll () derrors.Error {
	// Delete any namespace labelled with appinstanceid

	var finalResult derrors.Error = nil

	options := metav1.ListOptions {
		IncludeUninitialized: true,
	}

	// Get all namespaces
	nsList, err := m.KubernetesClient.CoreV1().Namespaces().List(options)
	if err != nil {
		log.Error().Err(err).Msg("error retrieving all namespaces")
		return conversions.ToDerror(err)
	}

	// Check if each namespace has the label "nalej-app-instance-id"
	for _, ns := range nsList.Items {
		toDelete := false
		if ns.Status.Phase != corev1.NamespaceTerminating {
			nsLabels := ns.GetLabels()
			for k, _ := range nsLabels {
				if k == utils.NALEJ_ANNOTATION_APP_INSTANCE_ID {
					toDelete = true
					break
				}
			}
		}

		if toDelete {
			// If it does: delete it
			log.Debug().Str("namespace", ns.Namespace).Msg("deleting namespace")
			err = m.KubernetesClient.CoreV1().Namespaces().Delete(ns.Name, metav1.NewDeleteOptions(DeleteGracePeriod))

			if err != nil {
				log.Error().Err(err).Msg("impossible to delete namespace")
				thisError := derrors.NewGenericError(fmt.Sprintf("impossible to delete namespace %s", ns.Name), err)

				if finalResult == nil {
					finalResult = thisError
				} else {
					finalResult = derrors.NewGenericError(thisError.StackToString(), thisError)
				}
			}
		}
	}

	return finalResult
}