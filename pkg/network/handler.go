/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
	"context"
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/derrors"
	pbCommon "github.com/nalej/grpc-common-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/nalej/grpc-network-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/rs/zerolog/log"
)

// Handler in charge of the ConductorNetwork services.

type Handler struct {
	mng *Manager
}

func NewHandler(m *Manager) *Handler {
	return &Handler{mng: m}
}

// A pod requests authorization to join a ZT Network
func (h *Handler) AuthorizeNetworkMembership(context context.Context, request *pbDeploymentMgr.AuthorizeNetworkMembershipRequest) (*pbCommon.Success, error) {
	log.Debug().Msgf("authorize member %s to join network %s", request.MemberId, request.NetworkId)

	err := h.mng.AuthorizeNetworkMembership(request.OrganizationId, request.AppInstanceId, request.ServiceGroupInstanceId,
		request.ServiceInstanceId, request.NetworkId, request.MemberId, request.IsProxy)
	if err != nil {
		msg := fmt.Sprintf("error authorizing member %s in network %s", request.MemberId, request.NetworkId)
		log.Error().Err(err).Msgf(msg)
		return nil, derrors.NewGenericError(msg, err)
	}

	return &pbCommon.Success{}, nil
}

// Request the creation of a new network entry
/*
func (h *Handler) RegisterNetworkEntry(context context.Context, request *pbDeploymentMgr.RegisterNetworkEntryRequest) (*pbCommon.Success, error) {
	log.Debug().Msgf("reqister network entry for app %s in organization %s with ip %s ", request.ServiceName,
		request.OrganizationId, request.ServiceIp)

	err := h.mng.RegisterNetworkEntry(request.OrganizationId, request.AppInstanceId,
		request.NetworkId, request.ServiceName, request.ServiceIp, request.ServiceGroupInstanceId, request.ServiceAppInstanceId)

	if err != nil {
		msg := fmt.Sprintf("error registering network entry request %#v", request)
		log.Error().Err(err).Msgf(msg)
		return nil, derrors.NewGenericError(msg, err)
	}

	return &pbCommon.Success{}, nil
}
*/
// SetServiceRoute setups an iptables DNAT for a given service
func (h *Handler) SetServiceRoute(context context.Context, request *pbDeploymentMgr.ServiceRoute) (*pbCommon.Success, error) {
	verr := ValidSetRoute(request)
	if verr != nil {
		return nil, conversions.ToGRPCError(verr)
	}
	err := h.mng.SetServiceRoute(request)
	if err != nil {
		return nil, conversions.ToGRPCError(err)
	}
	return &pbCommon.Success{}, nil
}

func (h *Handler) AuthorizeZTConnection(_ context.Context, request *grpc_network_go.AuthorizeZTConnectionRequest) (*pbCommon.Success, error){
	log.Debug().Interface("request", request).Msg("authorize ZT Connection request")

	vErr := entities.ValidateAuthorizeZTConnectionRequest(request)
	if vErr != nil {
		return nil, conversions.ToGRPCError(vErr)
	}

	err := h.mng.AuthorizeZTConnection(request)
	if err != nil {
		return nil, err
	}
	return &pbCommon.Success{}, nil
}
