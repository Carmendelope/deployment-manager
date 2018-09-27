/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
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