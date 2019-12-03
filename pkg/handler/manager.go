/*
 * Copyright 2019 Nalej
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
	"context"
	"errors"
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/internal/structures"
	"github.com/nalej/deployment-manager/internal/structures/monitor"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/network"
	"github.com/nalej/grpc-application-go"
	pbConductor "github.com/nalej/grpc-conductor-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/nalej/grpc-unified-logging-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
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
	DefaultTimeout      = time.Minute
	CheckSleepTime      = time.Second * 4
	ExpireTimeout       = time.Minute * 3
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
	// Network decorator
	networkDecorator     executor.NetworkDecorator
	// Unified Logging Slave address
	unifiedLoggingClient grpc_unified_logging_go.SlaveClient
	// Kubernetes Client
	NetUpdater           network.NetworkUpdater
}

func NewManager(
	executor *executor.Executor,
	clusterPublicHostname string,
	queue structures.RequestsQueue,
	dnsHosts []string, monitored monitor.MonitoredInstances,
	publicCredentials grpc_application_go.ImageCredentials,
	networkDecorator executor.NetworkDecorator,
	ulClient grpc_unified_logging_go.SlaveClient,
	K8sClient *kubernetes.Clientset) *Manager {
	netUpdater := network.NewKubernetesNetworkUpdater(K8sClient)
	return &Manager{
		executor:              *executor,
		clusterPublicHostname: clusterPublicHostname,
		dnsHosts:              dnsHosts,
		monitored:             monitored,
		queue:                 queue,
		PublicCredentials:     publicCredentials,
		networkDecorator:      networkDecorator,
		unifiedLoggingClient:  ulClient,
		NetUpdater:            netUpdater,
	}
}

func (m *Manager) Run() {
	sleep := time.Tick(time.Millisecond * CheckQueueSleepTime)
	for {
		select {
		case <-sleep:
			for m.queue.AvailableRequests() {
				log.Info().Int("queued requests", m.queue.Len()).Msg("there are pending deployment requests")
				go m.processRequest(m.queue.NextRequest())
				// give it a second
				time.Sleep(time.Second)
			}
		}
	}
}

func (m *Manager) processRequest(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
	log.Debug().Msgf("execute plan with id %s", request.RequestId)

	var executionError error

	// Check the existence of a namespace for this app
	namespace, executionError := m.executor.GetApplicationNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId, int(request.NumRetry))
	if executionError != nil {
		log.Error().Err(executionError).Msg("impossible to find a valid namespace name")
		return executionError
	}

	// Compute the namespace for this deployment
	// namespace := common.GetNamespace(request.Fragment.OrganizationId, request.Fragment.AppInstanceId, int(request.NumRetry))

	// Build a metadata object
	metadata := entities.DeploymentMetadata{
		Namespace:             namespace,
		AppDescriptorId:       request.Fragment.AppDescriptorId,
		AppDescriptorName:     request.Fragment.AppDescriptorName,
		AppInstanceId:         request.Fragment.AppInstanceId,
		ZtNetworkId:           request.ZtNetworkId,
		OrganizationName:      request.Fragment.OrganizationName,
		OrganizationId:        request.Fragment.OrganizationId,
		DeploymentId:          request.Fragment.DeploymentId,
		AppName:               request.Fragment.AppInstanceName,
		NalejVariables:        request.Fragment.NalejVariables,
		FragmentId:            request.Fragment.FragmentId,
		DNSHosts:              m.dnsHosts,
		ClusterPublicHostname: m.clusterPublicHostname,
		// ----
		PublicCredentials: m.PublicCredentials,
		// Set stage in each iteration
		//Stage:
	}

	preDeployable, executionError := m.executor.PrepareEnvironmentForDeployment(metadata, m.networkDecorator)
	if executionError != nil {
		log.Error().Err(executionError).Msgf("failed environment preparation for fragment %s",
			request.Fragment.FragmentId)
		log.Info().Msgf("undeploy deployments for preparation in fragment %s", request.Fragment.FragmentId)
		if preDeployable != nil {
			executionError = preDeployable.Undeploy()
			if executionError != nil {
				log.Error().Err(executionError).Msgf("impossible to undeploy preparation for fragment %s",
					request.Fragment.FragmentId)
			}
		} else {
			log.Info().Msg("there is no information to undeploy the object!!")
		}

		m.monitored.SetEntryStatus(request.Fragment.FragmentId, entities.FRAGMENT_ERROR, executionError)
		return errors.New(fmt.Sprintf("failed environment preparation for fragment %s",
			request.Fragment.FragmentId))
	}

	for stageNumber, stage := range request.Fragment.Stages {
		services := stage.Services
		log.Info().Msgf("plan %d contains %d services to execute", stageNumber, len(services))

		// fill the stage specific information
		metadata.Stage = *stage

		deployable, executionError := m.executor.BuildNativeDeployable(metadata, m.networkDecorator)

		if executionError != nil {
			log.Error().Err(executionError).Msgf("impossible to build deployment for fragment %s", request.Fragment.FragmentId)
			m.monitored.SetEntryStatus(request.Fragment.FragmentId, entities.FRAGMENT_ERROR, executionError)
			return executionError
		}

		// Add deployment stage to monitor entries
		// Platform resources will be filled by the corresponding deployables
		log.Info().Str("fragmentId", request.Fragment.FragmentId).Msg("add monitoring data")
		monitoringData := m.getMonitoringData(namespace, stage, request.Fragment)
		m.monitored.AddEntry(monitoringData)

		switch request.RollbackPolicy {
		case pbDeploymentMgr.RollbackPolicy_NONE:
			log.Info().Msgf("rollback policy was set to %s, stop any deployment", request.RollbackPolicy)
			executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, NoRetries)
		case pbDeploymentMgr.RollbackPolicy_ALWAYS_RETRY:
			log.Info().Msgf("rollback policy was set to %s, retry until done", request.RollbackPolicy)
			executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, UnlimitedStageRetries)
		case pbDeploymentMgr.RollbackPolicy_LIMITED_RETRY:
			log.Info().Msgf("rollback policy was set to %s, retry limited times", request.RollbackPolicy)
			executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, MaxStageRetries)
		default:
			log.Warn().Msgf("unknown rollback policy %s, no rollback by default", request.RollbackPolicy)
			executionError = m.deploymentLoopStage(request.Fragment, stage, deployable, namespace, NoRetries)
		}
		if executionError != nil {
			log.Error().AnErr("error", executionError).Int("stageNumber", stageNumber).
				Int("totalStages", len(request.Fragment.Stages)).Msg("error deploying stage")
			// clear fragment operations
			log.Info().Str("fragmentId", request.Fragment.FragmentId).Msg("clear fragment")

			err := preDeployable.Undeploy()
			if err != nil {
				log.Error().Err(err).Str("fragmentId", request.Fragment.FragmentId).Msgf("impossible to undeploy preparation for fragment")
			}
			m.monitored.SetEntryStatus(request.Fragment.FragmentId, entities.FRAGMENT_ERROR, executionError)
			return executionError
		}

		// Done
		log.Info().Msgf("executed fragment %s stage %d / %d", request.Fragment.FragmentId, stageNumber+1, len(request.Fragment.Stages))
	}
	return executionError
}

func (m *Manager) Execute(request *pbDeploymentMgr.DeploymentFragmentRequest) error {
	// push the request to the queue
	m.queue.PushRequest(request)
	return nil
}

// expireLogs send a message to unified-logging to expire the logs of an application
func (m *Manager) expireLogs(organizationId string, appInstanceId string) {

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, ulErr := m.unifiedLoggingClient.Expire(ctx, &grpc_unified_logging_go.ExpirationRequest{
		OrganizationId: organizationId,
		AppInstanceId:  appInstanceId,
	})
	if ulErr != nil {
		log.Warn().Str("error", conversions.ToDerror(ulErr).DebugReport()).Msg("error expiring logs")
	}
	log.Debug().Str("organizationID", organizationId).Str("instanceID", appInstanceId).Msg("logs expired")
}

// checkNamespaceToExpireLogs runs a loop to check when the application namespace is terminated
func (m *Manager) checkNamespaceToExpireLogs(request *pbDeploymentMgr.UndeployRequest) {

	sleep := time.NewTicker(CheckSleepTime)

	ctxExpire, cancelExpire := context.WithTimeout(context.Background(), ExpireTimeout)
	defer cancelExpire()

	for {
		select {
		case <-sleep.C:
			exists, err := m.NetUpdater.CheckIfNamespaceExists(request.OrganizationId, request.AppInstanceId)
			if err != nil {
				log.Warn().Str("organizationID", request.OrganizationId).Str("instanceID", request.AppInstanceId).
					Str("error", err.DebugReport()).Msg("Unable to expire logs")
			}
			if !exists {
				m.expireLogs(request.OrganizationId, request.AppInstanceId)
				sleep.Stop()
				return
			}
		case <-ctxExpire.Done():
			log.Warn().Str("organizationID", request.OrganizationId).Str("instanceID", request.AppInstanceId).Msg("Unable to expire logs, context deadline")
			return
		}
	}

}

func (m *Manager) Undeploy(request *pbDeploymentMgr.UndeployRequest) error {
	log.Debug().Str("appInstanceID", request.AppInstanceId).Msg("undeploy app instance with id")

	// Undeploy the namespace
	err := m.executor.UndeployNamespace(request, m.networkDecorator)
	// set the requested application as terminating
	m.monitored.SetAppStatus(request.AppInstanceId, entities.FRAGMENT_TERMINATING, nil)

	go m.checkNamespaceToExpireLogs(request)

	if err != nil {
		log.Error().Err(err).Msgf("impossible to undeploy app %s", request.AppInstanceId)
		return err
	}

	return nil
}

func (m *Manager) UndeployFragment(request *pbDeploymentMgr.UndeployFragmentRequest) error {
	// set this fragment as terminating
	entry := m.monitored.GetEntry(request.DeploymentFragmentId)
	if entry == nil {
		return errors.New(fmt.Sprintf("deployment fragment %s was not found to be undeployed", request.DeploymentFragmentId))
	}
	undeployErr := m.executor.UndeployFragment(entry.Namespace, request.DeploymentFragmentId)
	// remove the monitored entry
	m.monitored.SetEntryStatus(request.DeploymentFragmentId, entities.FRAGMENT_TERMINATING, nil)
	// if the application moves into terminating status remove the namespace
	appStatus, _ := m.monitored.GetAppStatus(request.AppInstanceId)

	log.Debug().Interface("appStatus", appStatus).Msg("checking the status of the app after undeploying fragment")

	undeployRequest := &pbDeploymentMgr.UndeployRequest{
		OrganizationId: request.OrganizationId,
		AppInstanceId:  request.AppInstanceId,
	}
	if appStatus == nil {
		m.executor.UndeployNamespace(undeployRequest, m.networkDecorator)
	} else if *appStatus == entities.FRAGMENT_TERMINATING {
		m.executor.UndeployNamespace(undeployRequest, m.networkDecorator)
	}

	return undeployErr
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
		m.monitored.SetEntryStatus(fragment.FragmentId, entities.FRAGMENT_DEPLOYING, nil)

		err := m.executor.DeployStage(toDeploy, fragment, stage)

		if err != nil {
			log.Error().Err(err).Msgf("there was a problem when retrying stage %s from fragment %s",
				stage.StageId, fragment.FragmentId)
			m.monitored.SetEntryStatus(fragment.FragmentId, entities.FRAGMENT_RETRYING, err)
		}

		log.Info().Str("namespace", namespace).Str("fragmentIdappInstanceId", fragment.AppInstanceId).
			Str("fragmentId", fragment.FragmentId).
			Str("stage", stage.StageId).Msg("wait for pending checks to finish")
		stageErr := m.monitored.WaitPendingChecks(fragment.FragmentId, StageCheckTime, StageCheckTimeout)
		log.Debug().Msg("Finished waiting for pending checks")

		if stageErr == nil {
			// Everything was OK, stage deployed
			return nil
		}

		// undeploy
		err = toDeploy.Undeploy()
		if err != nil {
			log.Error().Err(err).Msgf("there was a problem when undeploying stage %s from fragment %s",
				stage.StageId, fragment.FragmentId)
			return err
		}

		log.Info().Msgf("failed retry %d out of %d for stage %s in fragment %s", retries+1, maxRetries, stage.StageId, fragment.FragmentId)

		// It didn't work. Go into a retry loop
		time.Sleep(SleepBetweenRetries * time.Millisecond)
	}

	return errors.New(fmt.Sprintf("exceeded number of retries for stage %s in fragment %s", stage.StageId, fragment.FragmentId))
}

// Internal function that builds all the data structures to monitor a stage
//  params:
//   namespace
//   stage to be deployed
//   fragment
//  returns:
//   monitoring entry
func (m *Manager) getMonitoringData(namespace string, stage *pbConductor.DeploymentStage, fragment *pbConductor.DeploymentFragment) *entities.MonitoredAppEntry {
	services := make(map[string]*entities.MonitoredServiceEntry, 0)
	for _, s := range stage.Services {
		services[s.ServiceInstanceId] = &entities.MonitoredServiceEntry{
			FragmentId:             fragment.FragmentId,
			AppDescriptorId:        s.AppDescriptorId,
			AppInstanceId:          s.AppInstanceId,
			ServiceGroupInstanceId: s.ServiceGroupInstanceId,
			ServiceGroupId:         s.ServiceGroupId,
			OrganizationId:         s.OrganizationId,
			ServiceID:              s.ServiceId,
			ServiceName:            s.ServiceName,
			ServiceInstanceID:      s.ServiceInstanceId,
			Info:                   "",
			Endpoints:              make([]entities.EndpointInstance, 0),
			Status:                 entities.NALEJ_SERVICE_SCHEDULED,
			NewStatus:              true,
			Resources:              make(map[string]*entities.MonitoredPlatformResource, 0),
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
		Namespace:        namespace,
	}
	return toReturn
}
