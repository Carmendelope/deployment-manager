/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package handler

import (
    "errors"
    "fmt"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/derrors"
    "github.com/nalej/grpc-application-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    "github.com/rs/zerolog/log"
    "time"
)

const (
    // No rollback stage retries
    NoRetries = 1
    // Maximum number of retries for a stage rollback
    MaxStageRetries = 3
    // Unlimited retries
    UnlimitedStageRetries = -1
    // Sleep between retries in milliseconds
    SleepBetweenRetries = 10000

    // Time between sleeps to check if an application is up
    StageCheckTime = 10
    // Checkout time after a stage is considered to be failed
    StageCheckTimeout = 480
    // Time to wait between checks in the queue in milliseconds.
    CheckQueueSleepTime = 2000
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
    // Requests queue
    queue structures.RequestsQueue
    // Config
    PublicCredentials grpc_application_go.ImageCredentials
}

func NewManager(
    executor *executor.Executor,
    clusterPublicHostname string,
    queue structures.RequestsQueue,
    dnsHosts []string, monitored monitor.MonitoredInstances,
    publicCredentials grpc_application_go.ImageCredentials) *Manager {
        return &Manager{
        executor: *executor,
        clusterPublicHostname: clusterPublicHostname,
        dnsHosts: dnsHosts,
        monitored: monitored,
        queue: queue,
        PublicCredentials: publicCredentials,
    }
}


func(m *Manager) Run() {
    sleep := time.Tick(time.Millisecond * CheckQueueSleepTime)
    for {
        select {
        case <- sleep:
            for m.queue.AvailableRequests() {
                log.Info().Int("queued requests", m.queue.Len()).Msg("there are pending deployment requests")
                go m.processRequest(m.queue.NextRequest())
                // give it a second
                time.Sleep(time.Second)
            }
        }
    }
}

func(m *Manager) processRequest(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
    log.Debug().Msgf("execute plan with id %s",request.RequestId)
    // Compute the namespace for this deployment
    namespace := common.GetNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId, int(request.NumRetry))

    var executionError error

    // Add a new events controller for this application
    log.Info().Str("appInstanceId", request.Fragment.AppInstanceId).Msg("add monitoring controller for app")
    controller := m.executor.AddEventsController(request.Fragment.AppInstanceId, m.monitored, namespace)
    if controller == nil{
        err := errors.New(fmt.Sprintf("impossible to create deployment controller for namespace %s",namespace))
        log.Error().Err(err).Str("appInstanceId", request.Fragment.AppInstanceId).Msg("failed creating controller")
        //return err
        executionError = err
        m.monitored.SetAppStatus(request.Fragment.AppInstanceId,entities.FRAGMENT_ERROR,executionError)
        return executionError
    }

    // Build a metadata object
    metadata := entities.DeploymentMetadata{
        Namespace: namespace,
        AppDescriptorId: request.Fragment.AppDescriptorId,
        AppInstanceId: request.Fragment.AppInstanceId,
        ZtNetworkId: request.ZtNetworkId,
        OrganizationName: request.Fragment.OrganizationName,
        OrganizationId: request.Fragment.OrganizationId,
        DeploymentId: request.Fragment.DeploymentId,
        AppName: request.Fragment.AppName,
        NalejVariables: request.Fragment.NalejVariables,
        FragmentId: request.Fragment.FragmentId,
        DNSHosts: m.dnsHosts,
        ClusterPublicHostname: m.clusterPublicHostname,
        // ----
        PublicCredentials: m.PublicCredentials,
        // Set stage in each iteration
        //Stage:
    }

    preDeployable, executionError := m.executor.PrepareEnvironmentForDeployment(metadata)
    if executionError != nil {
        log.Error().Err(executionError).Msgf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId)
        log.Info().Msgf("undeploy deployments for preparation in fragment %s",request.Fragment.FragmentId)
        if preDeployable != nil {
            executionError = preDeployable.Undeploy()
            if executionError != nil {
                log.Error().Err(executionError).Msgf("impossible to undeploy preparation for fragment %s",
                    request.Fragment.FragmentId)
            }
        } else {
            log.Info().Msg("there is no information to undeploy the object!!")
        }

        m.monitored.SetAppStatus(request.Fragment.AppInstanceId, entities.FRAGMENT_ERROR,executionError)
        return errors.New(fmt.Sprintf("failed environment preparation for fragment %s",
            request.Fragment.FragmentId))
    }


    for stageNumber, stage := range request.Fragment.Stages {
        services := stage.Services
        log.Info().Msgf("plan %d contains %d services to execute",stageNumber, len(services))

        // fill the stage specific information
        metadata.Stage = *stage

        deployable, executionError := m.executor.BuildNativeDeployable(metadata)

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


func(m *Manager) Execute(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
    // push the request to the queue
    m.queue.PushRequest(request)
    return nil
}

func (m *Manager) Undeploy (request *pbDeploymentMgr.UndeployRequest) error {
	log.Debug().Str("appInstanceID", request.AppInstanceId).Msg("undeploy app instance with id")

    // Stop monitoring events
	// TODO check if this operation is really required
    //m.executor.StopControlEvents(request.AppInstanceId)

	// Undeploy the namespace
	err := m.executor.UndeployNamespace(request)
	// set the requested application as terminating
	m.monitored.SetAppStatus(request.AppInstanceId, entities.FRAGMENT_TERMINATING,nil)

	if err != nil {
		log.Error().Err(err).Msgf("impossible to undeploy app %s", request.AppInstanceId)
		return err
	}

    // Remove stage entries
    m.monitored.RemoveApp(request.AppInstanceId)


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

    // something happened. We reach the retry loop
    for retries := 0; retries < maxRetries; retries++ {
        m.monitored.SetAppStatus(fragment.AppInstanceId,entities.FRAGMENT_DEPLOYING,nil)

        // execute
        // Start controller here so we can consume already occurred events
        controller := m.executor.StartControlEvents(fragment.AppInstanceId)
        if controller == nil {
            return derrors.NewNotFoundError(fmt.Sprintf("no controller was found for appInstanceId %s",fragment.AppInstanceId))
        }

        err := m.executor.DeployStage(toDeploy, fragment, stage, m.monitored)

        if err != nil {
            log.Error().Err(err).Msgf("there was a problem when retrying stage %s from fragment %s",
                stage.StageId, fragment.FragmentId)
            m.monitored.SetAppStatus(fragment.AppInstanceId,entities.FRAGMENT_RETRYING,err)
        }

        log.Info().Str("namespace",namespace).Str("appInstanceId",fragment.AppInstanceId).
            Str("stage", stage.StageId).Msg("wait for pending checks to finish")
        stageErr := m.monitored.WaitPendingChecks(fragment.AppInstanceId, StageCheckTime, StageCheckTimeout)
        log.Debug().Msg("Finished waiting for pending checks")

        if stageErr == nil {
            // Everything was OK, stage deployed
            return nil
        }

        // Stop events control
        m.executor.StopControlEvents(fragment.AppInstanceId)

        // undeploy
        err = toDeploy.Undeploy()
        if err != nil {
            log.Error().Err(err).Msgf("there was a problem when undeploying stage %s from fragment %s",
                stage.StageId, fragment.FragmentId)
            return err
        }

        log.Info().Msgf("failed retry %d out of %d for stage %s in fragment %s", retries +1, maxRetries, stage.StageId, fragment.FragmentId)

        // It didn't work. Go into a retry loop
        time.Sleep(SleepBetweenRetries * time.Millisecond)
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
        services[s.ServiceInstanceId] = &entities.MonitoredServiceEntry{
            FragmentId:  fragment.FragmentId,
            AppDescriptorId: s.AppDescriptorId,
            AppInstanceId: s.AppInstanceId,
            ServiceGroupInstanceId: s.ServiceGroupInstanceId,
            ServiceGroupId: s.ServiceGroupId,
            OrganizationId: s.OrganizationId,
            ServiceID: s.ServiceId,
            ServiceName: s.Name,
            ServiceInstanceID: s.ServiceInstanceId,
            Info: "",
            Endpoints: make([]entities.EndpointInstance,0),
            Status: entities.NALEJ_SERVICE_SCHEDULED,
            NewStatus: true,
            Resources: make(map[string]*entities.MonitoredPlatformResource,0),
            // This will be increased by the resources
            NumPendingChecks: 0,
        }
    }
    totalNumberServices := 0
    for _, dep := range fragment.Stages {
        totalNumberServices = totalNumberServices + len(dep.Services)
    }
    toReturn := &entities.MonitoredAppEntry{
        OrganizationId:   fragment.OrganizationId,
        FragmentId:       fragment.FragmentId,
        AppDescriptorId:  fragment.AppDescriptorId,
        AppInstanceId:    fragment.AppInstanceId,
        DeploymentId:     fragment.DeploymentId,
        Status:           entities.NALEJ_SERVICE_SCHEDULED,
        Services:         services,
        NumPendingChecks: len(services),
        Info:             "",
        TotalServices:    totalNumberServices,
        NewStatus:        true,
    }
    return toReturn
}
