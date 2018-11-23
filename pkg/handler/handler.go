/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
    "github.com/nalej/derrors"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/rs/zerolog/log"
    "context"
    "errors"
)

type Handler struct {
    m *Manager
}

func NewHandler(m *Manager) *Handler {
    return &Handler{m}
}

func (h* Handler) Execute(context context.Context, request *pbDeploymentMgr.DeploymentFragmentRequest) (*pbDeploymentMgr.DeploymentFragmentResponse, error) {
    log.Debug().Msgf("requested to execute fragment %v",request)
    if request == nil {
        theError := errors.New("received nil deployment plan request")
        return nil, theError
    }

    if !h.ValidDeployFragmentRequest(request) {
        theError := errors.New("non valid deployment fragment request")
        return nil, theError
    }

    err := h.m.Execute(request)
    if err != nil {
        log.Error().Err(err).Msgf("failed to execute fragment request %s",request.RequestId)
        response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_ERROR}
        return &response, err
    }

    response := pbDeploymentMgr.DeploymentFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_RUNNING}

    return &response, nil
}

func (h* Handler) Undeploy (context context.Context, request *pbDeploymentMgr.UndeployRequest) derrors.Error {
    log.Debug().Msgf("requested to undeploy application %v", request)
    if request == nil {
        err := derrors.NewGenericError("received nil undeploy request")
        return err
    }

    if !h.ValidUndeployRequest(request) {
        err := derrors.NewGenericError("non valid undeploy request")
        return err
    }

    err := h.m.Undeploy(request)

    if err != nil {
       log.Error().Err(err).Msgf("failed to undeploy application %s",request.AppInstanceId)
       return err
    }

    return nil
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

func (h *Handler) ValidUndeployRequest (request *pbDeploymentMgr.UndeployRequest) bool {
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