/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package handler

import (
    "errors"
    "fmt"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "time"
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

)


type Manager struct {
    executor executor.Executor
    // Conductor address
    conductorAddress string
    // Cluster Public Hostname for the ingresses
    clusterPublicHostname string
    // Dns hosts
    dnsHosts []string
    // Structure controlling monitored instances
    monitored monitor.MonitoredInstances
}

func NewManager(
    clusterApiConnection *grpc.ClientConn, executor *executor.Executor,
    loginHelper *login_helper.LoginHelper, clusterPublicHostname string,
    dnsHosts []string, monitored monitor.MonitoredInstances) *Manager {
    return &Manager{
        executor: *executor,
        clusterPublicHostname: clusterPublicHostname,
        dnsHosts: dnsHosts,
        monitored: monitored,
    }
}



func(m *Manager) Execute(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
    log.Debug().Msgf("execute plan with id %s",request.RequestId)

    namespace := common.GetNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId)
    preDeployable, prepError := m.executor.PrepareEnvironmentForDeployment(request.Fragment, namespace,m.monitored)
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
        deployable, err := m.executor.BuildNativeDeployable(
            stage, namespace, request.Fragment.NalejVariables, request.ZtNetworkId, request.Fragment.OrganizationId,
            request.Fragment.OrganizationName,request.Fragment.DeploymentId, request.Fragment.AppInstanceId,
            request.Fragment.AppName, m.clusterPublicHostname, m.dnsHosts)

        if err != nil {
            log.Error().Err(err).Msgf("impossible to build deployment for fragment %s",request.Fragment.FragmentId)
            return err
        }

        // Add deployment stage to monitor entries
        // Platform resources will be filled by the corresponding deployables
        monitoringData := m.getMonitoringData(stage, request.Fragment)
        m.monitored.AddStage(monitoringData)

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
    return nil
}

func (m *Manager) Undeploy (request *pbDeploymentMgr.UndeployRequest) error {
	log.Debug().Msgf("undeploy app instance with id %s",request.AppInstanceId)

    targetNS := common.GetNamespace(request.OrganizationId, request.AppInstanceId)

    log.Info().Str("namespace", targetNS).Msg("stop events controller")
    m.executor.StopControlEvents(targetNS)
    // TODO remove stage entries

	err := m.executor.UndeployNamespace(request)
	if err != nil {
		log.Error().Err(err).Msgf("impossible to undeploy app %s", request.AppInstanceId)
		return err
	}

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
    err := m.executor.DeployStage(toDeploy, fragment, stage, m.monitored)
    if err == nil {
        // DONE
        return nil
    }

    //m.monitor.UpdateFragmentStatus(fragment.OrganizationId,fragment.DeploymentId, fragment.AppInstanceId,
    //    fragment.FragmentId, entities.FRAGMENT_RETRYING)


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
        err = m.executor.DeployStage(toDeploy, fragment, stage, m.monitored)

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

// Internal function that builds all the data structures to monitor a stage
//  params:
//   stage to be deployed
//  returns:
//   monitoring entry
func (m *Manager) getMonitoringData(stage *pbConductor.DeploymentStage, fragment *pbConductor.DeploymentFragment) entities.MonitoredStageEntry{
    services := make(map[string]entities.MonitoredServiceEntry,0)
    for  _,s := range stage.Services {
        services[s.ServiceId] = entities.MonitoredServiceEntry{
            FragmentId: fragment.FragmentId,
            InstanceId: fragment.AppInstanceId,
            OrganizationId: fragment.OrganizationId,
            Info: "",
            Endpoints: make([]string,0),
            Status: entities.NALEJ_SERVICE_DEPLOYING,
            NewStatus: true,
            Resources: make(map[string]entities.MonitoredPlatformResource,0),
            // This will be increased by the resources
            NumPendingChecks: 0,
            ServiceID: s.ServiceId,
        }
    }
    toReturn := entities.MonitoredStageEntry{
        OrganizationId: fragment.OrganizationId,
        FragmentId: fragment.FragmentId,
        InstanceId: fragment.AppInstanceId,
        StageID: stage.StageId,
        Services: services,
        NumPendingChecks: len(services),
    }
    return toReturn
}