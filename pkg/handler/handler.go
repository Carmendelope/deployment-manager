/*
 * Copyright 2018 Nalej
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

package handler


import (
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "context"
    "errors"
)

type Handler struct {
    m *Manager
}

func NewHandler(m *Manager) *Handler {
    return &Handler{m}
}

func (h* Handler) Execute(context context.Context, request *pbDeploymentMgr.DeployPlanRequest) (*pbConductor.DeploymentResponse, error) {
    if request == nil {
        theError := errors.New("received nil deployment plan request")
        return nil, theError
    }

    return h.m.Execute(request)
}