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

package istio

import (
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/derrors"
)

const(
    IstioLabelInjection = "istio-injection"
)

type IstioDecorator struct {
    // Istio client
    //Client *versionedclient.Clientset
}

func NewIstioDecorator() (executor.NetworkDecorator, derrors.Error){

    /*
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, derrors.NewInternalError("impossible to get local configuration for internal k8s client", err)
    }

    // Get a versioned Istio client
    ic, err := versionedclient.NewForConfig(config)
    if err != nil {
        log.Error().Err(err).Msg("impossible to build a local Istio client")
        return nil, derrors.NewInternalError("impossible to build a local Istio client", err)
    }

    return &IstioDecorator{Client: ic}, nil
    */
    return &IstioDecorator{}, nil


}


func (id *IstioDecorator) Build(aux executor.Deployable, args ...interface{}) derrors.Error {

    switch target := aux.(type) {
    // Process a namespace
    case *kubernetes.DeployableNamespace:
        return id.decorateNamespace(target)
    default:
        // nothing to do
        return nil
    }
    return nil


}

func (id *IstioDecorator) Deploy(aux executor.Deployable, args ...interface{}) derrors.Error {
   // Nothing to do
   return nil
}

    // Remove any unnecessary entries when a deployable element is removed.
func (id *IstioDecorator) Undeploy(aux executor.Deployable, args ...interface{}) derrors.Error {
    return nil
}


// Add the Istio labels required by any namespace to enable de network traffic injection.
// params:
//  namespace to be modified
// return:
//  error if any
func (id *IstioDecorator) decorateNamespace(namespace *kubernetes.DeployableNamespace) derrors.Error {
    namespace.Namespace.Labels[IstioLabelInjection] = "enabled"
    return nil
}


