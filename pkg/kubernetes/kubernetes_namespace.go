/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
    apiv1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    v12 "k8s.io/client-go/kubernetes/typed/core/v1"
    "k8s.io/client-go/kubernetes"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/pkg/common"
)

// Deployable namespace
// A namespace is associated with a fragment. This is a special case of deployable that is only intended to be
// executed before the fragment deployment starts.
//--------------------

type DeployableNamespace struct {
    // kubernetes Client
    client v12.NamespaceInterface
    // fragment this namespace is attached to
    fragmentId string
    // namespace name descriptor
    targetNamespace string
    // namespace
    namespace apiv1.Namespace
}

func NewDeployableNamespace(client *kubernetes.Clientset, fragmentId string, targetNamespace string) *DeployableNamespace {
    return &DeployableNamespace{
        client:          client.CoreV1().Namespaces(),
        fragmentId:      fragmentId,
        targetNamespace: targetNamespace,
        namespace:       apiv1.Namespace{},
    }
}

func(n *DeployableNamespace) GetId() string {
    return n.fragmentId
}

func(n *DeployableNamespace) Build() error {
    ns := apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n.targetNamespace}}
    n.namespace = ns
    return nil
}

func(n *DeployableNamespace) Deploy(controller executor.DeploymentController) error {
    retrieved, err := n.client.Get(n.targetNamespace, metav1.GetOptions{IncludeUninitialized: true})

    if retrieved.Name!="" {
        n.namespace = *retrieved
        log.Warn().Msgf("namespace %s already exists",n.targetNamespace)
        return nil
    }
    created, err := n.client.Create(&n.namespace)
    if err != nil {
        return err
    }
    log.Debug().Msgf("invoked namespace with uid %s", string(created.Namespace))
    n.namespace = *created
    // The namespace is a special case that covers all the Services
    controller.AddMonitoredResource(string(created.GetUID()), common.AllServices,n.fragmentId)
    return err
}

func (n *DeployableNamespace) exists() bool{
    _, err := n.client.Get(n.targetNamespace, metav1.GetOptions{IncludeUninitialized: true})
    return err == nil
}

func(n *DeployableNamespace) Undeploy() error {
    if !n.exists(){
        log.Warn().Str("targetNamespace", n.targetNamespace).Msg("Target namespace does not exists, considering undeploy successful")
        return nil
    }
    err := n.client.Delete(n.targetNamespace, metav1.NewDeleteOptions(DeleteGracePeriod))
    return err
}