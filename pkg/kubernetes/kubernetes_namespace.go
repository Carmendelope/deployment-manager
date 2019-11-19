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
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/rs/zerolog/log"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Seconds for a timeout when listing namespaces
const NamespaceListTimeout = 2

// Deployable Namespace
// A Namespace is associated with a fragment. This is a special case of deployable that is only intended to be
// executed before the fragment deployment starts.
//--------------------

type DeployableNamespace struct {
	// kubernetes Client
	client v12.NamespaceInterface
	// deployment Data
	data entities.DeploymentMetadata
	// Namespace
	Namespace apiv1.Namespace
	// network decorator object for deployments
	networkDecorator executor.NetworkDecorator
}

func NewDeployableNamespace(client *kubernetes.Clientset, data entities.DeploymentMetadata,
	networkDecorator executor.NetworkDecorator) *DeployableNamespace {
	return &DeployableNamespace{
		client:    client.CoreV1().Namespaces(),
		data:      data,
		Namespace: apiv1.Namespace{},
		networkDecorator: networkDecorator,
	}
}

func (n *DeployableNamespace) GetId() string {
	return n.data.Stage.StageId
}

func (n *DeployableNamespace) Build() error {
	ns := apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: n.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT: n.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:     n.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:      n.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:     n.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:            n.data.Stage.StageId,
			},
		},
	}
	n.Namespace = ns

	netErr := n.networkDecorator.Build(n)
	if netErr != nil {
		log.Error().Err(netErr).Msg("error running network decorator during namespace building")
	}

	return nil
}

func (n *DeployableNamespace) Deploy(controller executor.DeploymentController) error {
	retrieved, err := n.client.Get(n.data.Namespace, metav1.GetOptions{IncludeUninitialized: true})

	if retrieved.Name != "" {
		n.Namespace = *retrieved
		log.Warn().Msgf("Namespace %s already exists", n.data.Namespace)
		return nil
	}
	created, err := n.client.Create(&n.Namespace)
	if err != nil {
		return err
	}
	log.Debug().Msgf("invoked Namespace with uid %s", string(created.Namespace))
	n.Namespace = *created

	netErr := n.Deploy(controller)
	if netErr != nil {
		log.Error().Err(netErr).Msg("error running networking decorator during namespace deploy")
	}

	return err
}

func (n *DeployableNamespace) exists() bool {
	_, err := n.client.Get(n.data.Namespace, metav1.GetOptions{IncludeUninitialized: true})
	return err == nil
}

func (n *DeployableNamespace) Undeploy() error {

	// Delete any Namespace labelled with appinstanceid and organizationid

	// query existing namespaces
	queryString := fmt.Sprintf("%s=%s,%s=%s",
		utils.NALEJ_ANNOTATION_ORGANIZATION_ID, n.data.OrganizationId,
		utils.NALEJ_ANNOTATION_APP_INSTANCE_ID, n.data.AppInstanceId,
	)
	options := metav1.ListOptions{
		IncludeUninitialized: true,
		LabelSelector:        queryString,
	}

	list, err := n.client.List(options)
	if err != nil {
		return derrors.AsError(err, "impossible to undeploy Namespace")
	}

	for _, l := range list.Items {
		log.Debug().Str("Namespace", l.Namespace).Msg("execute undeploy for Namespace")
		err = n.client.Delete(l.Name, metav1.NewDeleteOptions(DeleteGracePeriod))
		if err != nil {
			log.Error().Err(err).Msg("impossible to deploy Namespace")
		}
	}

	return nil
}
