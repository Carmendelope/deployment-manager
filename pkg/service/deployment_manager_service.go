/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package service

import (
    "fmt"
    "github.com/nalej/deployment-manager/pkg"
    "github.com/nalej/deployment-manager/pkg/cluster-api"
    "github.com/nalej/deployment-manager/pkg/handler"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/deployment-manager/pkg/network"
    "github.com/nalej/deployment-manager/pkg/utils"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/nalej/grpc-utils/pkg/tools"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
    "net"
    "os"
    "strconv"
)

// Configuration structure
type Config struct {
    // listening port
    Port uint32
    // ClusterAPIAddress address
    ClusterAPIAddress string
    // DeploymentManager address
    DeploymentMgrAddress string
    // is kubernetes locally available
    Local bool
    // Username/email
    Email string
    // Password
    Password string
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

func setEnvironmentVars(config *Config) {
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

    pkg.DEPLOYMENT_MANAGER_ADDR = config.DeploymentMgrAddress
}


func NewDeploymentManagerService(config *Config) (*DeploymentManagerService, error) {

    setEnvironmentVars(config)

    exec, err := kubernetes.NewKubernetesExecutor(config.Local)
    if err != nil {
        log.Panic().Err(err).Msg("there was an error creating kubernetes client")
        panic(err.Error())
        return nil, err
    }

    // login
    ClusterAPILogin := cluster_api.NewLogin(utils.MANAGER_ClUSTER_IP, utils.MANAGER_CLUSTER_PORT)
    ClusterAPILogin.Login(config.Email, config.Password)

    // Build connection with conductor
    log.Debug().Msgf("connect with conductor at %s", config.ClusterAPIAddress)
    conn, err := grpc.Dial(config.ClusterAPIAddress, grpc.WithInsecure())
    if err != nil {
        log.Panic().Err(err).Msgf("impossible to connect with conductor at %s", config.ClusterAPIAddress)
        panic(err.Error())
        return nil, err
    }

    // Instantiate deployment manager service
    mgr := handler.NewManager(conn,&exec)

    // Build connection with networking manager
    log.Debug().Msgf("connect with network manager at %s", config.ClusterAPIAddress)
    connNet, err := grpc.Dial(config.ClusterAPIAddress, grpc.WithInsecure())
    if err != nil {
        log.Panic().Err(err).Msgf("impossible to connect with networking manager at %s", config.ClusterAPIAddress)
        panic(err.Error())
        return nil, err
    }

    // Instantiate network manager service
    net := network.NewManager(connNet)

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