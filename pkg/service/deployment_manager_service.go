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
    "github.com/nalej/deployment-manager/pkg/network"
    "github.com/nalej/deployment-manager/pkg/utils"
    "github.com/nalej/deployment-manager/pkg"
    "os"
    "strconv"
    "net"
    "fmt"
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
    // Manager for networking services
    net *network.Manager
    // Server for incoming requests
    server *tools.GenericGRPCServer
    // configuration
    configuration Config
}

// Set the values of the environment variables.
func setEnvironmentVars() {
    if pkg.MANAGER_CLUSTER_IP = os.Getenv(utils.MANAGER_ClUSTER_IP); pkg.MANAGER_CLUSTER_IP == "" {
        log.Fatal().Msgf("%s variable was not set", utils.MANAGER_ClUSTER_IP)
    }

    if pkg.MANAGER_CLUSTER_PORT = os.Getenv(utils.MANAGER_CLUSTER_PORT); pkg.MANAGER_CLUSTER_PORT == "" {
        log.Fatal().Msgf("%s variable was not set", utils.MANAGER_CLUSTER_PORT)
        _, err :=  strconv.Atoi(pkg.MANAGER_CLUSTER_PORT)
        if err != nil {
            log.Fatal().Msgf("%s must be a port number", utils.MANAGER_CLUSTER_PORT)
        }
    }

}

func NewDeploymentManagerService(config *Config) (*DeploymentManagerService, error) {

    exec, err := kubernetes.NewKubernetesExecutor(config.Local)
    if err != nil {
        log.Panic().Err(err).Msg("there was an error creating kubernetes client")
        panic(err.Error())
        return nil, err
    }

    // Build connection with conductor
    conn, err := grpc.Dial(config.AddressConductor, grpc.WithInsecure())
    if err != nil {
        log.Panic().Err(err).Msgf("impossible to connect with system model at %s", config.AddressConductor)
        panic(err.Error())
        return nil, err
    }

    // Instantiate deployment manager service
    mgr := handler.NewManager(conn,&exec)

    // Instantiate network manager service
    net := network.NewManager(conn)

    // Instantiate target server
    server := tools.NewGenericGRPCServer(config.Port)


    instance := DeploymentManagerService{mgr: mgr, net: net, server: server, configuration: *config}

    return &instance, nil
}


func (d *DeploymentManagerService) Run() {
    // register services

    lis, err := net.Listen("tcp", fmt.Sprintf(":%d", d.configuration.Port))
    if err != nil {
        log.Fatal().Errs("failed to listen: %v", []error{err})
    }

    deployment := handler.NewHandler(d.mgr)
    network := network.NewHandler(d.net)

    // register
    grpcServer := grpc.NewServer()
    pbDeploymentMgr.RegisterDeploymentManagerServer(grpcServer, deployment)
    pbDeploymentMgr.RegisterDeploymentManagerNetworkServer(grpcServer, network)

    reflection.Register(grpcServer)
    // Run
    log.Info().Uint32("port", d.configuration.Port).Msg("Launching gRPC server")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatal().Errs("failed to serve: %v", []error{err})
    }

}