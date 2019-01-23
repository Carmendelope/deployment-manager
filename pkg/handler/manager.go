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
    "github.com/nalej/derrors"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/rs/zerolog/log"
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

    // Time between sleeps to check if an application is up
    StageCheckTime = 10
    // Checkout time after a stage is considered to be failed
    StageCheckTimeout = 240
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
    executor *executor.Executor,
    clusterPublicHostname string,
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
    // Compute the namespace for this deployment
    namespace := common.GetNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId)

    var executionError error

    // Add a new events controller
    log.Info().Str("namespace", namespace).Msg("add monitoring controller for namespace")
    controller := m.executor.AddEventsController(namespace, m.monitored)
    if controller == nil{
        err := errors.New(fmt.Sprintf("impossible to create deployment controller for namespace %s",namespace))
        log.Error().Err(err).Str("namespace", namespace).Msg("failed creating controller")
        //return err
        executionError = err
        m.monitored.SetAppStatus(request.Fragment.AppInstanceId,entities.FRAGMENT_ERROR,executionError)
        return executionError
    }

    preDeployable, executionError := m.executor.PrepareEnvironmentForDeployment(request.Fragment, namespace,m.monitored)
    if executionError != nil {
        log.Error().Err(executionError).Msgf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId)
        log.Info().Msgf("undeploy deployments for preparation in fragment %s",request.Fragment.FragmentId)
        executionError = preDeployable.Undeploy()
        if executionError != nil {
            log.Error().Err(executionError).Msgf("impossible to undeploy preparation for fragment %s",
                request.Fragment.FragmentId)
        }
        m.monitored.SetAppStatus(request.Fragment.AppInstanceId, entities.FRAGMENT_ERROR,executionError)
        return errors.New(fmt.Sprintf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId))
    }


    for stageNumber, stage := range request.Fragment.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))
        deployable, executionError := m.executor.BuildNativeDeployable(
            stage, namespace, request.Fragment.NalejVariables, request.ZtNetworkId, request.Fragment.OrganizationId,
            request.Fragment.OrganizationName,request.Fragment.DeploymentId, request.Fragment.AppInstanceId,
            request.Fragment.AppName, m.clusterPublicHostname, m.dnsHosts)

        if executionError != nil {
            log.Error().Err(executionError).Msgf("impossible to build deployment for fragment %s",request.Fragment.FragmentId)
            m.monitored.SetAppStatus(request.Fragment.AppInstanceId,entities.FRAGMENT_ERROR,executionError)
            return executionError
        }

        // Add deployment stage to monitor entries
        // Platform resources will be filled by the corresponding deployables
        log.Info().Str("appInstanceID",request.Fragment.AppInstanceId).Msg("add monitoring data")
        monitoringData := m.getMonitoringData(stage, request.Fragment)
        m.monitored.AddApp(monitoringData)

        switch request.RollbackPolicy {
            case pbDeploymentMgr.RollbackPolicy_NONE:
                log.Info().Msgf("rollback policy was set to %s, stop any deployment", request.RollbackPolicy)
                executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, NoRetries)
            case pbDeploymentMgr.RollbackPolicy_ALWAYS_RETRY:
                log.Info().Msgf("rollback policy was set to %s, retry until done", request.RollbackPolicy)
                executionError = m.deploymentLoopStage(request.Fragment,stage, deployable, namespace, UnlimitedStageRetries)
            case pbDeploymentMgr.RollbackPolicy_LIMITED_RETRY:
                log.Info().Msgf("rollback policy was set to %s, retry limited times", request.RollbackPolicy)
                executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, MaxStageRetries)
            default:
                log.Warn().Msgf("unknown rollback policy %s, no rollback by default", request.RollbackPolicy)
                executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, NoRetries)
        }
        if executionError != nil {
            log.Error().AnErr("error",executionError).Int("stageNumber",stageNumber).
                Int("totalStages",len(request.Fragment.Stages)).Msg("error deploying stage")
            // clear fragment operations
            log.Info().Str("fragmentId", request.Fragment.FragmentId).Msg("clear fragment")

            err := preDeployable.Undeploy()
            if err != nil {
                log.Error().Err(err).Str("fragmentId", request.Fragment.FragmentId).Msgf("impossible to undeploy preparation for fragment")
            }
            m.monitored.SetAppStatus(request.Fragment.AppInstanceId,entities.FRAGMENT_ERROR,executionError)
            return executionError
        }

        // Done
        log.Info().Msgf("executed fragment %s stage %d / %d",request.Fragment.FragmentId, stageNumber+1, len(request.Fragment.Stages))
    }
    return executionError
}

func (m *Manager) Undeploy (request *pbDeploymentMgr.UndeployRequest) error {
	log.Debug().Str("appInstanceID", request.AppInstanceId).Msg("undeploy app instance with id")

    targetNS := common.GetNamespace(request.OrganizationId, request.AppInstanceId)

    log.Info().Str("namespace", targetNS).Msg("stop events controller")
    m.executor.StopControlEvents(targetNS)

    // Remove stage entries
    m.monitored.RemoveApp(request.AppInstanceId)


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
//   namespace name of the target namespace
//   maxRetries is the maximum number of retries to be done, -1 to retry indefinitely
//  return:
//   error if any
func (m *Manager) deploymentLoopStage(fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage,
    toDeploy executor.Deployable, namespace string, maxRetries int) error {

    // Start controller here so we can consume already occurred events
    controller := m.executor.StartControlEvents(namespace)
    if controller == nil {
        return derrors.NewNotFoundError(fmt.Sprintf("no controller was found for namespace %s",namespace))
    }


    m.monitored.SetAppStatus(fragment.AppInstanceId,entities.FRAGMENT_DEPLOYING,nil)

    // first attempt
    err := m.executor.DeployStage(toDeploy, fragment, stage, m.monitored)
    if err != nil {
        return err
    }

    // run the controller
    log.Info().Str("namespace",namespace).Str("appInstanceId",fragment.AppInstanceId).
        Str("stage", stage.StageId).Msg("wait for pending checks to finish")
    stageErr := m.monitored.WaitPendingChecks(fragment.AppInstanceId, StageCheckTime, StageCheckTimeout)
    log.Debug().Msg("Finished waiting for pending checks")

    if stageErr == nil {
        // Everything was OK, stage deployed
        return nil
    }

    // something happened. We reach the retry loop
    for retries := 0; retries < maxRetries; retries++ {
        m.monitored.SetAppStatus(fragment.AppInstanceId,entities.FRAGMENT_RETRYING,stageErr)

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
        // Start controller here so we can consume already occurred events
        controller := m.executor.StartControlEvents(namespace)
        if controller == nil {
            return derrors.NewNotFoundError(fmt.Sprintf("no controller was found for namespace %s",namespace))
        }

        err = m.executor.DeployStage(toDeploy, fragment, stage, m.monitored)

        if err != nil {
            log.Error().Err(err).Msgf("there was a problem when retrying stage %s from fragment %s",
                stage.StageId, fragment.FragmentId)
        }

        log.Info().Str("namespace",namespace).Str("appInstanceId",fragment.AppInstanceId).
            Str("stage", stage.StageId).Msg("wait for pending checks to finish")
        stageErr := m.monitored.WaitPendingChecks(fragment.AppInstanceId, StageCheckTime, StageCheckTimeout)
        log.Debug().Msg("Finished waiting for pending checks")

        if stageErr == nil {
            // Everything was OK, stage deployed
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
func (m *Manager) getMonitoringData(stage *pbConductor.DeploymentStage, fragment *pbConductor.DeploymentFragment) *entities.MonitoredAppEntry{
    services := make(map[string]*entities.MonitoredServiceEntry,0)
    for  _,s := range stage.Services {
        services[s.ServiceId] = &entities.MonitoredServiceEntry{
            FragmentId: fragment.FragmentId,
            InstanceId: fragment.AppInstanceId,
            OrganizationId: fragment.OrganizationId,
            Info: "",
            Endpoints: make([]string,0),
            Status: entities.NALEJ_SERVICE_SCHEDULED,
            NewStatus: true,
            Resources: make(map[string]*entities.MonitoredPlatformResource,0),
            // This will be increased by the resources
            NumPendingChecks: 0,
            ServiceID: s.ServiceId,
        }
    }
    totalNumberServices := 0
    for _, dep := range fragment.Stages {
        totalNumberServices = totalNumberServices + len(dep.Services)
    }
    toReturn := &entities.MonitoredAppEntry{
        OrganizationId: fragment.OrganizationId,
        FragmentId: fragment.FragmentId,
        InstanceId: fragment.AppInstanceId,
        DeploymentId: fragment.DeploymentId,
        Status: entities.NALEJ_SERVICE_SCHEDULED,
        Services: services,
        NumPendingChecks: len(services),
        Info: "",
        TotalServices: totalNumberServices,
    }
    return toReturn
}
