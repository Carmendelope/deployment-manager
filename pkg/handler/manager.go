/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package handler

import (
    "github.com/nalej/deployment-manager/pkg/executor"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg/monitor"
    "github.com/nalej/deployment-manager/internal/entities"
    "fmt"
    "time"
    "errors"
)

const (
    // No rollback stage retries
    NoRetries = 0
    // Maximum number of retries for a stage rollback
    MaxStageRetries = 3
    // Unlimited retries
    UnlimitedStageRetries = -1
    // Sleep between retries in milliseconds
    SleepBetweenRetries = 10000
    // Maximum namespace length
    NamespaceLength = 63
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

    m.monitor.UpdateFragmentStatus(request.Fragment.OrganizationId,request.Fragment.DeploymentId,
        request.Fragment.FragmentId, request.Fragment.AppInstanceId, entities.FRAGMENT_DEPLOYING)

    namespace := m.getNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId)
    preDeployable, prepError := m.executor.PrepareEnvironmentForDeployment(request.Fragment, namespace, m.monitor)
    if prepError != nil {
        log.Error().Err(prepError).Msgf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId)
        log.Info().Msgf("undeploy deployments for preparation in fragment %s",request.Fragment.FragmentId)
        err := preDeployable.Undeploy()
        if err != nil {
            log.Error().Err(err).Msgf("impossible to undeploy preparation for fragment %s",
                request.Fragment.FragmentId)
            return errors.New("failed environment preparation for fragment %s, " +
                "impossible to undeploy preparation for fragment %s")
        }
        return errors.New(fmt.Sprintf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId))
    }

    for stageNumber, stage := range request.Fragment.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))
        deployable, err := m.executor.BuildNativeDeployable(stage, namespace, request.ZtNetworkId)

        if err != nil {
            log.Error().Err(err).Msgf("impossible to build deployment for fragment %s",request.Fragment.FragmentId)
            return err
        }

        var executionErr error
        switch request.RollbackPolicy {
            case pbDeploymentMgr.RollbackPolicy_NONE:
                log.Info().Msgf("rollback policy was set to %s, stop any deployment", request.RollbackPolicy)
                executionErr = m.deploymentLoopStage(request.Fragment, stage, deployable, NoRetries)
            case pbDeploymentMgr.RollbackPolicy_ALWAYS_RETRY:
                log.Info().Msgf("rollback policy was set to %s, retry until done", request.RollbackPolicy)
                executionErr = m.deploymentLoopStage(request.Fragment,stage, deployable, UnlimitedStageRetries)
            case pbDeploymentMgr.RollbackPolicy_LIMITED_RETRY:
                log.Info().Msgf("rollback policy was set to %s, retry limited times", request.RollbackPolicy)
                executionErr = m.deploymentLoopStage(request.Fragment, stage, deployable, MaxStageRetries)
            default:
                log.Warn().Msgf("unknown rollback policy %s, no rollback by default", request.RollbackPolicy)
                executionErr = m.deploymentLoopStage(request.Fragment, stage, deployable, NoRetries)
        }
        if executionErr != nil {
            log.Error().AnErr("error",err).Msgf("error deploying stage %d out of %d",stageNumber,
                len(request.Fragment.Stages))
            m.monitor.UpdateFragmentStatus(request.Fragment.OrganizationId,request.Fragment.DeploymentId,
                request.Fragment.FragmentId, request.Fragment.AppInstanceId, entities.FRAGMENT_ERROR)
            // clear fragment operations
            log.Info().Msgf("clear fragment %s", request.Fragment.FragmentId)

            err := preDeployable.Undeploy()
            if err != nil {
                log.Error().Err(err).Msgf("impossible to undeploy preparation for fragment %s",
                    request.Fragment.FragmentId)
            }

            return executionErr
        }

        // Done
        log.Info().Msgf("executed fragment %s stage %d / %d",request.Fragment.FragmentId, stageNumber+1, len(request.Fragment.Stages))
    }
    m.monitor.UpdateFragmentStatus(request.Fragment.OrganizationId,request.Fragment.DeploymentId,
        request.Fragment.FragmentId, request.Fragment.AppInstanceId, entities.FRAGMENT_DONE)
    return nil
}

// Private function to execute a stage in a loop of retries.
//  params:
//   fragment this stage belongs to
//   stage to be executed
//   toDeploy deployable objects
//   maxRetries is the maximum number of retries to be done, -1 to retry indefinitely
//  return:
//   error if any
func (m *Manager) deploymentLoopStage(fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage,
    toDeploy executor.Deployable, maxRetries int) error {

    // first attempt
    err := m.executor.DeployStage(toDeploy, fragment, stage, m.monitor)
    if err == nil {
        // DONE
        return nil
    }

    m.monitor.UpdateFragmentStatus(fragment.OrganizationId,fragment.DeploymentId, fragment.AppInstanceId,
        fragment.FragmentId, entities.FRAGMENT_RETRYING)


    for retries := 0; retries < maxRetries; retries++ {
        // It didn't work. Go into a retry loop
        time.Sleep(SleepBetweenRetries * time.Millisecond)

        // undeploy
        err = toDeploy.Undeploy()
        if err != nil {
            log.Error().Err(err).Msgf("there was a problem when undeploying stage %s from fragment %s",
                stage.StageId, fragment.FragmentId)
            return err
        }
        // execute
        err = m.executor.DeployStage(toDeploy, fragment, stage, m.monitor)

        if err != nil {
            log.Error().Err(err).Msgf("there was a problem when retrying stage %s from fragment %s",
                stage.StageId, fragment.FragmentId)
        } else {
            // DONE
            return nil
        }
        log.Info().Msgf("failed retry %d out of %d for stage %s in fragment %s", retries +1, maxRetries, stage.StageId, fragment.FragmentId)
    }

    return errors.New(fmt.Sprintf("exceeded number of retries for stage %s in fragment %s", stage.StageId, fragment.FragmentId))
}

// Return the namespace associated with a service.
//  params:
//   organizationId
//   appInstanceId
//  return:
//   associated namespace
func (m *Manager) getNamespace(organizationId string, appInstanceId string) string {
    target := fmt.Sprintf("%s-%s", organizationId, appInstanceId)
    // check if the namespace is larger than the allowed k8s namespace length
    if len(target) > NamespaceLength {
        return target[:NamespaceLength]
    }
    return target
}
