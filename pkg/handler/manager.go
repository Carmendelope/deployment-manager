/*
 * Copyright 2018 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
    "github.com/nalej/deployment-manager/pkg/executor"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
)

type Manager struct {
    executor executor.Executor
}

func NewManager(executor *executor.Executor) *Manager {
    return &Manager{executor: *executor}
}

func(m *Manager) Execute(request *pbDeploymentMgr.DeployPlanRequest) (*pbConductor.DeploymentResponse, error) {
    log.Debug().Msgf("execute plan with id %s",request.RequestId)

    for stageNumber, stage := range request.Plan.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))
        err := m.executor.Execute(stage)

        if err != nil {
            // TODO what to do if rollback fails
            log.Error().AnErr("error",err).Msgf("error deploying stage %d out of %d",stageNumber,len(request.Plan.Stages))
            m.executor.StageRollback(stage)
            error_response := pbConductor.DeploymentResponse{RequestId: request.RequestId, Status: pbConductor.ApplicationStatus_ERROR}
            return &error_response, err
        }
        log.Info().Msgf("executed plan %s stage %d / %d",request.Plan.DeploymentId, stageNumber, len(request.Plan.Stages))
    }

    ok_reponse := pbConductor.DeploymentResponse{RequestId: request.RequestId, Status: pbConductor.ApplicationStatus_RUNNING}
    return &ok_reponse, nil
}

