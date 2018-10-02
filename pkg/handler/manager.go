/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
    "github.com/nalej/deployment-manager/pkg/executor"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/rs/zerolog/log"
)

type Manager struct {
    executor executor.Executor
}

func NewManager(executor *executor.Executor) *Manager {
    return &Manager{executor: *executor}
}

func(m *Manager) Execute(request *pbDeploymentMgr.DeployFragmentRequest) error {
    log.Debug().Msgf("execute plan with id %s",request.RequestId)

    for stageNumber, stage := range request.Fragment.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))
        _,err := m.executor.Execute(request.Fragment, stage)

        if err != nil {
            // TODO decide what to do if rollback fails
            log.Error().AnErr("error",err).Msgf("error deploying stage %d out of %d",stageNumber,len(request.Fragment.Stages))
            m.executor.StageRollback(stage)
            return err
        }
        log.Info().Msgf("executed fragment %s stage %d / %d",request.Fragment.FragmentId, stageNumber+1, len(request.Fragment.Stages))
    }
    return nil
}

