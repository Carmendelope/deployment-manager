/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
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

// Deployable namespace
// A namespace is associated with a fragment. This is a special case of deployable that is only intended to be
// executed before the fragment deployment starts.
//--------------------

type DeployableNamespace struct {
    // kubernetes Client
    client v12.NamespaceInterface
    // deployment data
    data entities.DeploymentMetadata
    // namespace
    namespace apiv1.Namespace
}

func NewDeployableNamespace(client *kubernetes.Clientset, data entities.DeploymentMetadata) *DeployableNamespace {
    return &DeployableNamespace{
        client:          client.CoreV1().Namespaces(),
        data:            data,
        namespace:       apiv1.Namespace{},
    }
}

func(n *DeployableNamespace) GetId() string {
    return n.data.Stage.StageId
}

func(n *DeployableNamespace) Build() error {
    ns := apiv1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            Name: n.data.Namespace,
            Labels: map[string]string{
                utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT : n.data.FragmentId,
                utils.NALEJ_ANNOTATION_ORGANIZATION : n.data.OrganizationId,
                utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : n.data.AppDescriptorId,
                utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : n.data.AppInstanceId,
                utils.NALEJ_ANNOTATION_STAGE_ID : n.data.Stage.StageId,
            },
        },
    }
    n.namespace = ns
    return nil
}

func(n *DeployableNamespace) Deploy(controller executor.DeploymentController) error {
    retrieved, err := n.client.Get(n.data.Namespace, metav1.GetOptions{IncludeUninitialized: true})

    if retrieved.Name!="" {
        n.namespace = *retrieved
        log.Warn().Msgf("namespace %s already exists",n.data.Namespace)
        return nil
    }
    created, err := n.client.Create(&n.namespace)
    if err != nil {
        return err
    }
    log.Debug().Msgf("invoked namespace with uid %s", string(created.Namespace))
    n.namespace = *created
    return err
}

func (n *DeployableNamespace) exists() bool{
    _, err := n.client.Get(n.data.Namespace, metav1.GetOptions{IncludeUninitialized: true})
    return err == nil
}

func(n *DeployableNamespace) Undeploy() error {

    // Delete any namespace labelled with appinstanceid and organizationid

    // query existing namespaces
    queryString := fmt.Sprintf("%s=%s,%s=%s",
        utils.NALEJ_ANNOTATION_ORGANIZATION, n.data.OrganizationId,
        utils.NALEJ_ANNOTATION_APP_INSTANCE_ID, n.data.AppInstanceId,
        )
    options := metav1.ListOptions {
        IncludeUninitialized: true,
        LabelSelector: queryString,
    }

    list, err := n.client.List(options)
    if err != nil {
        return derrors.AsError(err, "impossible to undeploy namespace")
    }

    for _, l := range list.Items {
        log.Debug().Str("namespace", l.Namespace).Msg("execute undeploy for namespace")
        err = n.client.Delete(l.Name, metav1.NewDeleteOptions(DeleteGracePeriod))
        if err != nil {
            log.Error().Err(err).Msg("impossible to deploy namespace")
        }
    }

    return nil
}