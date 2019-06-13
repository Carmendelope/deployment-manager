/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package service

import (
    "crypto/tls"
    "fmt"
    "github.com/nalej/deployment-manager/internal/structures"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/config"
    "github.com/nalej/deployment-manager/pkg/handler"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    monitor2 "github.com/nalej/deployment-manager/pkg/monitor"
    "github.com/nalej/deployment-manager/pkg/network"
    "github.com/nalej/deployment-manager/pkg/proxy"
    "github.com/nalej/derrors"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/nalej/grpc-utils/pkg/tools"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/reflection"
    "net"
    "strings"
)

type DeploymentManagerService struct {
    // Manager with the logic for incoming requests
    mgr *handler.Manager
    // Manager for networking services
    net *network.Manager
    // Proxy manager for proxy forwarding
    netProxy *proxy.Manager
    // Server for incoming requests
    server *tools.GenericGRPCServer
    // configuration
    configuration config.Config
}

func getClusterAPIConnection(hostname string, port int) (*grpc.ClientConn, derrors.Error) {
    // Build connection with cluster API
    tlsConfig := &tls.Config{
        ServerName:   hostname,
        InsecureSkipVerify: true,
    }
    targetAddress := fmt.Sprintf("%s:%d", hostname, port)
    log.Debug().Str("address", targetAddress).Msg("creating cluster API connection")

    creds := credentials.NewTLS(tlsConfig)

    log.Debug().Interface("creds", creds.Info()).Msg("Secure credentials")
    sConn, dErr := grpc.Dial(targetAddress, grpc.WithTransportCredentials(creds))
    if dErr != nil {
        return nil, derrors.AsError(dErr, "cannot create connection with the cluster API service")
    }
    return sConn, nil
}

func NewDeploymentManagerService(cfg *config.Config) (*DeploymentManagerService, error) {

    rErr := cfg.Resolve()
    if rErr != nil {
        log.Fatal().Str("trace", rErr.DebugReport()).Msg("cannot resolve variables")
    }

    vErr := cfg.Validate()
    if vErr != nil {
        log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
    }

    cfg.Print()
    config.SetGlobalConfig(cfg)

    // login
    clusterAPILoginHelper := login_helper.NewLogin(cfg.LoginHostname, int(cfg.LoginPort), cfg.UseTLSForLogin, cfg.Email, cfg.Password)
    err := clusterAPILoginHelper.Login()
    if err != nil {
        log.Panic().Err(err).Msg("there was an error requesting cluster-api login")
        panic(err.Error())
        return nil, err
    }

    exec, kubErr := kubernetes.NewKubernetesExecutor(cfg.Local, cfg.PlanetPath)

    if kubErr != nil {
        log.Panic().Err(err).Msg("there was an error creating kubernetes client")
        panic(err.Error())
        return nil, kubErr
    }

    // Build connection with conductor
    log.Debug().Str("hostname", cfg.ClusterAPIHostname).Msg("connecting with cluster api")
    clusterAPIConn, errCond := getClusterAPIConnection(cfg.ClusterAPIHostname, int(cfg.ClusterAPIPort))
    if errCond != nil {
        log.Panic().Err(err).Str("hostname", cfg.ClusterAPIHostname).Msg("impossible to connect with cluster api")
        panic(err.Error())
        return nil, errCond
    }

    log.Info().Msg("instantiate memory based instances monitor structure...")
    instanceMonitor := monitor.NewMemoryMonitoredInstances()
    log.Info().Msg("done")

    log.Info().Msg("start monitor helper service...")
    monitorService := monitor2.NewMonitorHelper(clusterAPIConn,clusterAPILoginHelper, instanceMonitor)
    go monitorService.Run()
    log.Info().Msg("done")

    nalejDNSForPods := strings.Split(cfg.DNS, ",")
    nalejDNSForPods = append(nalejDNSForPods, "8.8.8.8")

    // Instantiate a memory queue for requests
    requestsQueue := structures.NewMemoryRequestQueue()
    // Instantiate deployment manager service
    log.Info().Msg("star deployment requests manager")
    mgr := handler.NewManager(&exec, cfg.ClusterPublicHostname, requestsQueue, nalejDNSForPods, instanceMonitor, cfg.PublicCredentials)
    go mgr.Run()
    log.Info().Msg("done")

    // Instantiate network manager service
    net := network.NewManager(clusterAPIConn, clusterAPILoginHelper)

    // Instantiate app network manager service
    netProxy := proxy.NewManager(clusterAPIConn, clusterAPILoginHelper)

    // Instantiate target server
    server := tools.NewGenericGRPCServer(cfg.Port)

    instance := DeploymentManagerService{mgr: mgr, net: net, netProxy: netProxy, server: server, configuration: *cfg}

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
    netProxy := proxy.NewHandler(d.netProxy)

    // register
    grpcServer := grpc.NewServer()
    pbDeploymentMgr.RegisterDeploymentManagerServer(grpcServer, deployment)
    pbDeploymentMgr.RegisterDeploymentManagerNetworkServer(grpcServer, network)
    pbDeploymentMgr.RegisterApplicationProxyServer(grpcServer, netProxy)

    if d.configuration.Debug{
        reflection.Register(grpcServer)
    }

    // Run
    log.Info().Uint32("port", d.configuration.Port).Msg("Launching gRPC server")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatal().Errs("failed to serve: %v", []error{err})
    }
}
