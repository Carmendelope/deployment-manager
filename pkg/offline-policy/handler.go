/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package offline_policy

import (
	"context"
	grpc_common_go "github.com/nalej/grpc-common-go"
)

type Handler struct {
	Manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{Manager: manager}
}

func (h *Handler) RemoveAll (ctx context.Context, req *grpc_common_go.Empty) (*grpc_common_go.Success, error) {
	err := h.Manager.RemoveAll()
	if err !=  nil {
		return nil, err
	}
	return &grpc_common_go.Success{}, nil
}
