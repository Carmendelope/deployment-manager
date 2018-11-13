/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    pbCommon "github.com/nalej/grpc-common-go"
    "context"
    "github.com/rs/zerolog/log"
    "github.com/nalej/derrors"
    "fmt"
)


// Handler in charge of the ConductorNetwork services.

type Handler struct {
    mng *Manager
}

func NewHandler(m *Manager) *Handler {
    return &Handler{mng: m}
}

// A pod requests authorization to join a ZT Network
func(h *Handler) AuthorizeNetworkMembership(context context.Context, request *pbDeploymentMgr.AuthorizeNetworkMembershipRequest) (*pbCommon.Success, error) {
    err := h.mng.AuthorizeNetworkMembership(request.OrganizationId, request.NetworkId, request.MemberId)
    if err != nil {
        msg := fmt.Sprintf("error authorizing member %s in network %s", request.MemberId, request.NetworkId)
        log.Error().Err(err).Msgf(msg)
        return nil, derrors.NewGenericError(msg,err)
    }

    return &pbCommon.Success{}, nil
}

// Request the creation of a new network entry
func (h *Handler) RegisterNetworkEntry(context context.Context, request *pbDeploymentMgr.RegisterNetworkEntryRequest) (*pbCommon.Success, error) {

    log.Debug().Msgf("%#v", request)

    err := h.mng.RegisterNetworkEntry(request.OrganizationId, request.NetworkId, request.ServiceName, request.ServiceIp)
    if err != nil {
        msg := fmt.Sprintf("error registering network entry request %#v", request)
        log.Error().Err(err).Msgf(msg)
        return nil, derrors.NewGenericError(msg,err)
    }

    return &pbCommon.Success{}, nil
}



