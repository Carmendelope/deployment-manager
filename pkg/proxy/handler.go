package proxy

import (
    "github.com/nalej/grpc-common-go"
    "github.com/nalej/grpc-network-go"
    "context"
)

/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

type Handler struct {
     Manager *Manager
 }

func NewHandler(manager *Manager) *Handler {
    return &Handler{Manager: manager}
}

// RegisterInboundServiceProxy operation to update rules based on new service proxy being created.
func (h *Handler) RegisterInboundServiceProxy(context.Context, *grpc_network_go.InboundServiceProxy) (*grpc_common_go.Success, error) {
    return nil, nil
}
// RegisterOutboundProxy operation to retrieve existing networking rules.
func (h *Handler) RegisterOutboundProxy(context.Context, *grpc_network_go.OutboundService) (*grpc_common_go.Success, error) {
    return nil, nil
}