/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
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

func (h* Handler) Execute(context context.Context, request *pbDeploymentMgr.DeployFragmentRequest) (*pbDeploymentMgr.DeployFragmentResponse, error) {
    if request == nil {
        theError := errors.New("received nil deployment plan request")
        return nil, theError
    }

    err := h.m.Execute(request)
    if err != nil {
        log.Error().Err(err).Msgf("failed to execute fragment request %s",request.RequestId)
        response := pbDeploymentMgr.DeployFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_ERROR}
        return &response, err
    }

    response := pbDeploymentMgr.DeployFragmentResponse{RequestId: request.RequestId, Status: pbApplication.ApplicationStatus_RUNNING}

    return &response, nil
}