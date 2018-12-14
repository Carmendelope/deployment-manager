/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
	"context"
	"errors"
	"github.com/nalej/deployment-manager/internal/entities"
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

	/*
	err := h.m.Execute(request)
	if err != nil {
		log.Error().Err(err).Str("requestId", request.RequestId).Msg("failed to execute fragment request")
		res := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_ERROR}
		// TODO include error description in message
		return &res, err
	}

	response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_RUNNING}
	log.Debug().Interface("executeResult", response).Msg("executed fragment responds")
	*/

	go h.triggerExecute(request)

	response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_DEPLOYING}
	return &response, nil
}

func (h * Handler) triggerExecute(request *pbDeploymentMgr.DeploymentFragmentRequest) {
	log.Debug().Str("requestId", request.RequestId).Str("fragmentId", request.Fragment.FragmentId).Msg("triggerExecute starts")
	err := h.m.Execute(request)
	fragment := request.Fragment
	if err != nil {
		log.Error().Err(err).Str("requestId", request.RequestId).Msg("failed to execute fragment request")
		//res := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_ERROR}
		h.m.monitor.UpdateFragmentStatus(fragment.OrganizationId, fragment.DeploymentId, fragment.AppInstanceId,
			fragment.FragmentId, entities.FRAGMENT_ERROR)

		// TODO include error description in message
		return
	}

	response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_RUNNING}
	log.Debug().Interface("executeResult", response).Msg("executed fragment responds")

	log.Debug().Str("requestId", request.RequestId).Str("fragmentId", request.Fragment.FragmentId).Msg("triggerExecute finishes")
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
