package proxy

import (
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/derrors"
    "github.com/nalej/grpc-cluster-api-go"
    "github.com/nalej/grpc-network-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    grpc_status "google.golang.org/grpc/status"
)

/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

type Manager struct {
    // Client
    Client grpc_cluster_api_go.NetworkManagerClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewManager(conn *grpc.ClientConn, clusterApiLoginHelper *login_helper.LoginHelper) *Manager {
    client := grpc_cluster_api_go.NewNetworkManagerClient(conn)
    return &Manager{Client: client, ClusterAPILoginHelper: clusterApiLoginHelper}
}

func (m *Manager) RegisterInboundServiceProxy (request *grpc_network_go.InboundServiceProxy) derrors.Error {
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.RegisterInboundServiceProxy(ctx, request)

    if err != nil {
        st := grpc_status.Convert(err).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, err = m.Client.RegisterInboundServiceProxy(ctx2, request)
        } else {
            log.Error().Err(err).Msgf("error updating service status")
        }
    }

    if err != nil {
        return derrors.NewGenericError(err.Error())
    }

    return nil
}

func (m *Manager) RegisterOutboundProxy(request *grpc_network_go.OutboundService) derrors.Error {
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.RegisterOutboundProxy(ctx, request)

    if err != nil {
        st := grpc_status.Convert(err).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, err = m.Client.RegisterOutboundProxy(ctx2, request)
        } else {
            log.Error().Err(err).Msgf("error updating service status")
        }
    }

    if err != nil {
        return derrors.NewGenericError(err.Error())
    }

    return nil
}