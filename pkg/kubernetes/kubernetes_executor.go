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
	"errors"
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	pbConductor "github.com/nalej/grpc-conductor-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sync"
)

// The executor is the main structure in charge of running deployment plans on top of K8s.
type KubernetesExecutor struct {
	Client *kubernetes.Clientset
	// Controller that handles Kubernetes events add updates the MonitoredInstance
	Controller executor.DeploymentController
	// mutex
	mu sync.Mutex
}

func NewKubernetesExecutor(internal bool, controller executor.DeploymentController) (executor.Executor, error) {
	var c *kubernetes.Clientset
	var err error

	if internal {
		c, err = getInternalKubernetesClient()
	} else {
		c, err = getExternalKubernetesClient()
	}
	if err != nil {
		log.Error().Err(err).Msg("impossible to create kubernetes clientset")
		return nil, err
	}
	if c == nil {
		foundError := errors.New("kubernetes clientset was nil")
		log.Error().Err(foundError)
		return nil, foundError
	}

	// TBD Create events controller

	toReturn := KubernetesExecutor{
		Client:     c,
		Controller: controller,
	}
	return &toReturn, err
}

func (k *KubernetesExecutor) BuildNativeDeployable(metadata entities.DeploymentMetadata) (executor.Deployable, error) {

	log.Debug().Msgf("fragment %s stage %s requested to be translated into K8s deployable",
		metadata.FragmentId, metadata.Stage.StageId)

	var resources executor.Deployable
	k8sDeploy := NewDeployableKubernetesStage(k.Client, metadata)
	resources = k8sDeploy

	err := k8sDeploy.Build()

	if err != nil {
		log.Error().Err(err).Msgf("impossible to build resources for stage %s in fragment %s",
			metadata.Stage.StageId, metadata.FragmentId)
		return nil, err
	}

	log.Debug().Interface("metadata", metadata).Interface("k8sDeployable", resources).Msg("built k8s deployable")

	return resources, nil
}

func (k *KubernetesExecutor) GetApplicationNamespace(organizationId string, appInstanceId string, numRetry int) (string, error) {
	// Find the namespace
	ns := k.Client.CoreV1().Namespaces()
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{utils.NALEJ_ANNOTATION_ORGANIZATION_ID: organizationId,
		utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: appInstanceId}}
	opts := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	list, err := ns.List(opts)
	if err != nil {
		log.Error().Err(err).Str("organizationId", organizationId).Str("appInstanceId", appInstanceId).
			Msg("error when querying the application namespace")
		return "", err
	}

	// we iterate until we find a ready namespace ignoring any terminating namespace
	for _, potentialTarget := range list.Items {
		if potentialTarget.Status.Phase == v1.NamespaceActive {
			// this namespace is ok
			return potentialTarget.Name, nil
		}
	}

	// There is no namespace available return a new name
	return common.GetNamespace(organizationId, appInstanceId, numRetry), nil
}

// Prepare the namespace for the deployment. This is a special case because all the Deployments will share a common
// namespace. If this step cannot be done, no stage deployment will start.
func (k *KubernetesExecutor) PrepareEnvironmentForDeployment(metadata entities.DeploymentMetadata) (executor.Deployable, error) {
	log.Debug().Str("fragmentId", metadata.FragmentId).Msg("prepare environment for deployment")

	// Create a namespace
	namespaceDeployable := NewDeployableNamespace(k.Client, metadata)
	err := namespaceDeployable.Build()
	if err != nil {
		log.Error().Err(err).Msgf("impossible to build namespace %s", metadata.Namespace)
		return nil, err
	}

	if !namespaceDeployable.exists() {
		log.Debug().Str("namespace", metadata.Namespace).Msg("create namespace...")
		// TODO Check if namespace already exists...
		err = namespaceDeployable.Deploy(k.Controller)
		if err != nil {
			log.Error().Err(err).Msgf("impossible to deploy namespace %s", metadata.Namespace)
			return nil, err
		}
		log.Debug().Str("namespace", metadata.Namespace).Msg("namespace... created")

		// NP-766. create the nalej-public-registry on the user namespace
		nalejSecret := NewDeployableNalejSecret(k.Client, metadata)
		err = nalejSecret.Build()
		if err != nil {
			log.Error().Err(err).Msg("impossible to build nalej-public-registry secret")
		}
		err = nalejSecret.Deploy(k.Controller)
		if err != nil {
			log.Error().Err(err).Msg("impossible to deploy nalej-public-registry secret")
			return nil, err
		}
	} else {
		log.Debug().Str("namespace", metadata.Namespace).Msg("namespace already exists... skip")
	}

	var toReturn executor.Deployable
	toReturn = namespaceDeployable

	return toReturn, nil
}

// Deploy a stage into kubernetes. This function
func (k *KubernetesExecutor) DeployStage(toDeploy executor.Deployable, fragment *pbConductor.DeploymentFragment,
	stage *pbConductor.DeploymentStage) error {
	log.Info().Str("stage", stage.StageId).Msgf("execute stage %s with %d Services", stage.StageId, len(stage.Services))

	// Deploy everything and then start the controller.
	err := toDeploy.Deploy(k.Controller)
	if err != nil {
		log.Error().Err(err).Msgf("impossible to deploy resources for stage %s in fragment %s", stage.StageId, stage.FragmentId)
		return err
	}

	return err
}

func (k *KubernetesExecutor) UndeployStage(stage *pbConductor.DeploymentStage, toUndeploy executor.Deployable) error {
	log.Info().Msgf("undeploy stage %s from fragment %s", stage.StageId, stage.FragmentId)
	err := toUndeploy.Undeploy()
	if err != nil {
		log.Error().Msgf("error undeploying stage %s from fragment %s", stage.StageId, stage.FragmentId)
	}
	return err
}

// TODO reorganize this function to use balance the code among the different deployable entities
func (k *KubernetesExecutor) UndeployFragment(namespace string, fragmentId string) error {
	log.Info().Msgf("undeploy fragment %s in namespace %s", fragmentId, namespace)

	deleteOptions := metav1.DeleteOptions{}
	queryOptions := metav1.ListOptions{LabelSelector: fmt.Sprintf("nalej-deployment-fragment=%s", fragmentId)}

	// deployments
	err := k.Client.AppsV1().Deployments(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// replica sets
	err = k.Client.AppsV1().ReplicaSets(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// config maps
	err = k.Client.CoreV1().ConfigMaps(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// ingress
	err = k.Client.CoreV1().Endpoints(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// load balancers
	err = k.Client.CoreV1().PersistentVolumeClaims(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// secrets
	err = k.Client.CoreV1().Secrets(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// jobs
	err = k.Client.BatchV1().Jobs(namespace).DeleteCollection(&deleteOptions, queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	// services
	list, err := k.Client.CoreV1().Services(namespace).List(queryOptions)
	if err != nil {
		log.Error().Err(err).Msg("error undeploying fragments")
	}
	for _, x := range list.Items {
		err = k.Client.CoreV1().Services(namespace).Delete(x.Name, &deleteOptions)
		if err != nil {
			log.Error().Err(err).Msg("error undeploying fragments")
		}
	}

	return nil
}

func (k *KubernetesExecutor) UndeployNamespace(request *pbDeploymentMgr.UndeployRequest) error {
	// TODO check if this operation can remove iterative namespaces 0-XXXXX, 1-XXXXX, etc
	// A partially filled namespace object should be enough to find the target namespace
	metadata := entities.DeploymentMetadata{OrganizationId: request.OrganizationId, AppInstanceId: request.AppInstanceId}

	ns := NewDeployableNamespace(k.Client, metadata)
	err := ns.Build()
	if err != nil {
		log.Error().Msgf("error building deployable namespace %s", ns.namespace.Name)
		return err
	}

	err = ns.Undeploy()
	if err != nil {
		return err
	}

	return nil
}

func (k *KubernetesExecutor) StageRollback(stage *pbConductor.DeploymentStage, deployed executor.Deployable) error {
	log.Info().Msgf("requested rollback for stage %s", stage.StageId)
	// Call the undeploy for this deployable
	err := deployed.Undeploy()
	if err != nil {
		return err
	}

	return nil
}

// Helping function for pointer conversion.
func int32Ptr(i int32) *int32 { return &i }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }
