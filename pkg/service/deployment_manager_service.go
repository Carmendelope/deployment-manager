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

package service

import (
    "github.com/nalej/deployment-manager/pkg/handler"
    "github.com/nalej/deployment-manager/tools"
    "github.com/nalej/deployment-manager/pkg/executor"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "google.golang.org/grpc/reflection"
)

type DeploymentManagerService struct {
    // Manager with the logic for incoming requests
    mgr *handler.Manager
    // Server for incoming requests
    server *tools.GenericGRPCServer
}


func NewDeploymentManagerService(port uint32, executor *executor.Executor) (*DeploymentManagerService, error) {
    mgr := handler.NewManager(executor)
    deploymentServer := tools.NewGenericGRPCServer(port)

    instance := DeploymentManagerService{mgr: mgr, server: deploymentServer}

    return &instance, nil
}


func (d *DeploymentManagerService) Run() {
    // register services
    deployment := handler.NewHandler(d.mgr)
    pbDeploymentMgr.RegisterDeploymentManagerServer(d.server.Server, deployment)
    reflection.Register(d.server.Server)
    // Run
    d.server.Run()

}