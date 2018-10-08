/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package service

import (
    "github.com/nalej/deployment-manager/pkg/handler"
    "github.com/nalej/grpc-utils/pkg/tools"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "google.golang.org/grpc/reflection"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
)

// Configuration structure
type Config struct {
    // listening port
    Port uint32
    // Conductor address
    AddressConductor string
    // is kubernetes locally available
    Local bool
}

type DeploymentManagerService struct {
    // Manager with the logic for incoming requests
    mgr *handler.Manager
    // Server for incoming requests
    server *tools.GenericGRPCServer
}


func NewDeploymentManagerService(config *Config) (*DeploymentManagerService, error) {

    exec, err := kubernetes.NewKubernetesExecutor(config.Local)
    if err != nil {
        log.Panic().Err(err).Msg("there was an error creating kubernetes client")
        panic(err.Error())
        return nil, err
    }

    // Build connection with system model
    conn, err := grpc.Dial(config.AddressConductor, grpc.WithInsecure())
    if err != nil {
        log.Panic().Err(err).Msgf("impossible to connect with system model at %s", config.AddressConductor)
        panic(err.Error())
        return nil, err
    }

    mgr := handler.NewManager(conn,&exec)
    deploymentServer := tools.NewGenericGRPCServer(config.Port)

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