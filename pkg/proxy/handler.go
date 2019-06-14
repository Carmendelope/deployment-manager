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
func (h *Handler) RegisterInboundServiceProxy(ctx context.Context, request *grpc_network_go.InboundServiceProxy) (*grpc_common_go.Success, error) {
    err := h.Manager.RegisterInboundServiceProxy(request)
    if err !=  nil {
        return nil, err
    }
    return &grpc_common_go.Success{}, nil
}
// RegisterOutboundProxy operation to retrieve existing networking rules.
func (h *Handler) RegisterOutboundProxy(ctx context.Context, request *grpc_network_go.OutboundService) (*grpc_common_go.Success, error) {
    err := h.Manager.RegisterOutboundProxy(request)
    if err !=  nil {
        return nil, err
    }
    return &grpc_common_go.Success{}, nil
}