/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
	"context"
	"errors"
	pbApplication "github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-common-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	m *Manager
}

func NewHandler(m *Manager) *Handler {
	return &Handler{m}
}

func (h *Handler) Execute(context context.Context, request *pbDeploymentMgr.DeploymentFragmentRequest) (*pbDeploymentMgr.DeploymentFragmentResponse, error) {
	log.Debug().Interface("request", request).Msgf("requested to execute fragment")
	if request == nil {
		theError := errors.New("received nil deployment plan request")
		return nil, theError
	}

	if !h.ValidDeployFragmentRequest(request) {
		theError := errors.New("non valid deployment fragment request")
		return nil, theError
	}

	// Execute operation will take control now in an asynchronous manner
	h.m.Execute(request)

	response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_DEPLOYING}
	return &response, nil
}



func (h *Handler) Undeploy(context context.Context, request *pbDeploymentMgr.UndeployRequest) (*grpc_common_go.Success, error) {
	log.Debug().Interface("request", request).Msg("requested to undeploy application")
	if request == nil {
		err := errors.New("received nil undeploy request")
		return nil, err
	}

	if !h.ValidUndeployRequest(request) {
		err := errors.New("non valid undeploy request")
		return nil, err
	}

	err := h.m.Undeploy(request)

	if err != nil {
		log.Error().Err(err).Str("appInstanceId", request.AppInstanceId).Msg("failed to undeploy application")
		return nil, err
	}

	return &grpc_common_go.Success{}, nil
}

func (h *Handler) UndeployFragment(context context.Context, request *pbDeploymentMgr.UndeployFragmentRequest) (*grpc_common_go.Success, error) {
	log.Debug().Interface("request", request).Msg("requested to undeploy fragment")
	if request == nil {
		err := errors.New("received nil undeploy request")
		return nil, err
	}

	if !h.ValidUndeployFragmentRequest(request) {
		err := errors.New("non valid undeploy request")
		return nil, err
	}

	err := h.m.UndeployFragment(request)

	if err != nil {
		log.Error().Err(err).Str("fragmentId", request.DeploymentFragmentId).Msg("failed to undeploy fragment")
		return nil, err
	}

	return &grpc_common_go.Success{}, nil
}

func (h *Handler) ValidDeployFragmentRequest(request *pbDeploymentMgr.DeploymentFragmentRequest) bool {
	if request.RequestId == "" {
		log.Error().Msg("impossible to process request with no request_id")
		return false
	}
	if request.ZtNetworkId == "" {
		log.Error().Msg("impossible to process request with no zt_network_id")
		return false
	}
	if request.Fragment == nil || request.Fragment.FragmentId == "" {
		log.Error().Msg("impossible to process request with no fragment_id")
		return false
	}
	if request.Fragment.DeploymentId == "" || request.Fragment.AppInstanceId == "" || request.Fragment.OrganizationId == "" {
		log.Error().Msg("impossible to process request with no deployment_id, app_intance_id or organization_id")
		return false
	}

	return true
}

func (h *Handler) ValidUndeployRequest(request *pbDeploymentMgr.UndeployRequest) bool {
	if request.OrganizationId == "" {
		log.Error().Msg("impossible to process request with no organization_id")
		return false
	}
	if request.AppInstanceId == "" {
		log.Error().Msg("impossible to process request with no app_instance_id")
		return false
	}

	return true
}

func (h *Handler) ValidUndeployFragmentRequest(request *pbDeploymentMgr.UndeployFragmentRequest) bool {
	if request.OrganizationId == "" {
		log.Error().Msg("impossible to process request with no organization_id")
		return false
	}
	if request.AppInstanceId == "" {
		log.Error().Msg("impossible to process request with no app_instance_id")
		return false
	}
	if request.DeploymentFragmentId == "" {
		log.Error().Msg("impossible to process request with no deployment_fragment_id")
		return false
	}


	return true
}
