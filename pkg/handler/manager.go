/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package handler

import (
    "github.com/nalej/deployment-manager/pkg/executor"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg/monitor"
    "github.com/nalej/deployment-manager/internal/entities"
)

type Manager struct {
    executor executor.Executor
    // Helper monitor
    monitor *monitor.MonitorHelper
}

func NewManager(conductorConnection *grpc.ClientConn, executor *executor.Executor) *Manager {
    monitor := monitor.NewMonitorHelper(conductorConnection)
    return &Manager{executor: *executor, monitor: monitor}
}

func(m *Manager) Execute(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
    log.Debug().Msgf("execute plan with id %s",request.RequestId)

    m.monitor.UpdateFragmentStatus(request.Fragment.FragmentId, entities.FRAGMENT_DEPLOYING)
    for stageNumber, stage := range request.Fragment.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))
        deployed,err := m.executor.Execute(request.Fragment, stage, m.monitor)

        if err != nil {
            // TODO decide what to do if rollback fails
            log.Error().AnErr("error",err).Msgf("error deploying stage %d out of %d",stageNumber,len(request.Fragment.Stages))
            m.monitor.UpdateFragmentStatus(request.Fragment.FragmentId, entities.FRAGMENT_ERROR)
            m.executor.StageRollback(stage,*deployed)
            return err
        }
        // Done
        log.Info().Msgf("executed fragment %s stage %d / %d",request.Fragment.FragmentId, stageNumber+1, len(request.Fragment.Stages))
    }
    m.monitor.UpdateFragmentStatus(request.Fragment.FragmentId, entities.FRAGMENT_DONE)
    return nil
}

