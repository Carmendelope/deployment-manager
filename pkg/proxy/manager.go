package proxy

import (
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/derrors"
    "github.com/nalej/grpc-network-go"
    "google.golang.org/grpc"
)

/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

type Manager struct {
    // Client
    Client grpc_network_go.ApplicationNetworkClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewManager(conn *grpc.ClientConn, clusterApiLoginHelper *login_helper.LoginHelper) *Manager {
    client := grpc_network_go.NewApplicationNetworkClient(conn)
    return &Manager{Client: client, ClusterAPILoginHelper: clusterApiLoginHelper}
}

func (m *Manager) RegisterInboundServiceProxy (request *grpc_network_go.InboundServiceProxy) derrors.Error {
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.RegisterInboundServiceProxy(ctx, request)
    if err != nil {
        return derrors.NewInternalError("impossible to forward inbound service proxy request")
    }

    return nil
}

func (m *Manager) RegisterOutboundProxy(request *grpc_network_go.OutboundService) derrors.Error {
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.RegisterOutboundProxy(ctx, request)
    if err != nil {
        return derrors.NewInternalError("impossible to forward outbound service proxy request")
    }

    return nil
}