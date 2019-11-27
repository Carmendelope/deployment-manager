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

package entities

import (
	"github.com/nalej/derrors"
	pbApplication "github.com/nalej/grpc-application-go"
	pbConductor "github.com/nalej/grpc-conductor-go"
	"github.com/nalej/grpc-deployment-manager-go"
	"github.com/nalej/grpc-network-go"
	apps_v1 "k8s.io/api/apps/v1"
)

// Service status definition

type NalejServiceStatus int

const (
	NALEJ_SERVICE_SCHEDULED = iota
	NALEJ_SERVICE_WAITING
	NALEJ_SERVICE_DEPLOYING
	NALEJ_SERVICE_RUNNING
	NALEJ_SERVICE_ERROR
	NALEJ_SERVICE_TERMINATING
)

var ServiceStatusToGRPC = map[NalejServiceStatus]pbApplication.ServiceStatus{
	NALEJ_SERVICE_SCHEDULED:   pbApplication.ServiceStatus_SERVICE_SCHEDULED,
	NALEJ_SERVICE_WAITING:     pbApplication.ServiceStatus_SERVICE_WAITING,
	NALEJ_SERVICE_DEPLOYING:   pbApplication.ServiceStatus_SERVICE_DEPLOYING,
	NALEJ_SERVICE_RUNNING:     pbApplication.ServiceStatus_SERVICE_RUNNING,
	NALEJ_SERVICE_ERROR:       pbApplication.ServiceStatus_SERVICE_ERROR,
	NALEJ_SERVICE_TERMINATING: pbApplication.ServiceStatus_SERVICE_TERMINATING,
}

// Equivalence table between status services and their corresponding fragment status.
var ServicesToFragmentStatus = map[NalejServiceStatus]FragmentStatus{
	NALEJ_SERVICE_SCHEDULED:   FRAGMENT_WAITING,
	NALEJ_SERVICE_WAITING:     FRAGMENT_WAITING,
	NALEJ_SERVICE_DEPLOYING:   FRAGMENT_DEPLOYING,
	NALEJ_SERVICE_RUNNING:     FRAGMENT_DONE,
	NALEJ_SERVICE_ERROR:       FRAGMENT_ERROR,
	NALEJ_SERVICE_TERMINATING: FRAGMENT_TERMINATING,
}

var FragmentStatusToNalejServiceStatus = map[FragmentStatus]NalejServiceStatus{
	FRAGMENT_WAITING:     NALEJ_SERVICE_WAITING,
	FRAGMENT_TERMINATING: NALEJ_SERVICE_TERMINATING,
	// We assume that retrying is equivalent to deploying
	FRAGMENT_RETRYING:  NALEJ_SERVICE_DEPLOYING,
	FRAGMENT_DEPLOYING: NALEJ_SERVICE_DEPLOYING,
	FRAGMENT_ERROR:     NALEJ_SERVICE_ERROR,
	FRAGMENT_DONE:      NALEJ_SERVICE_RUNNING,
}

// Translate a kubenetes deployment status into a Nalej service status
// Kubernetes defines a set of deployment condition statuses to describe the current status of a deployment
// 	DeploymentAvailable
//	DeploymentProgressing
//	DeploymentReplicaFailure
// We ran the following conversion:
//  At least one into failure  -> error
//  Some applications progressing -> Deploying
//  All into available -> Running
//  Unknown situation --> Waiting
//
func KubernetesDeploymentStatusTranslation(kStatus apps_v1.DeploymentStatus) NalejServiceStatus {
	var result NalejServiceStatus
	running := 0
	progressing := 0
	error := 0
	for _, c := range kStatus.Conditions {
		switch c.Type {
		case apps_v1.DeploymentAvailable:
			running = running + 1
		case apps_v1.DeploymentProgressing:
			progressing = progressing + 1
		default:
			// this is a failure
			error = error + 1
		}
	}
	if error > 0 {
		result = NALEJ_SERVICE_ERROR
	} else if len(kStatus.Conditions) == 0 {
		// If no conditions were specified, we consider it as deploying
		result = NALEJ_SERVICE_DEPLOYING
	} else if running == len(kStatus.Conditions) {
		result = NALEJ_SERVICE_RUNNING
	} else if progressing > 0 {
		result = NALEJ_SERVICE_DEPLOYING
	} else {
		result = NALEJ_SERVICE_WAITING
	}

	// log.Debug().Msgf("translate condition status %v into %v",kStatus.Conditions, result)

	return result
}

// Translate the Nalej exposed service definition into the K8s service definition.

// Deployment fragment status definition

type FragmentStatus int

const (
	FRAGMENT_WAITING = iota
	FRAGMENT_DEPLOYING
	FRAGMENT_DONE
	FRAGMENT_ERROR
	FRAGMENT_RETRYING
	FRAGMENT_TERMINATING
)

var FragmentStatusToGRPC = map[FragmentStatus]pbConductor.DeploymentFragmentStatus{
	FRAGMENT_WAITING:     pbConductor.DeploymentFragmentStatus_WAITING,
	FRAGMENT_DEPLOYING:   pbConductor.DeploymentFragmentStatus_DEPLOYING,
	FRAGMENT_DONE:        pbConductor.DeploymentFragmentStatus_DONE,
	FRAGMENT_ERROR:       pbConductor.DeploymentFragmentStatus_ERROR,
	FRAGMENT_RETRYING:    pbConductor.DeploymentFragmentStatus_RETRYING,
	FRAGMENT_TERMINATING: pbConductor.DeploymentFragmentStatus_TERMINATING,
}

// Deployment metadata
type DeploymentMetadata struct {
	FragmentId            string                         `json:"fragment_id,omitempty"`
	Stage                 pbConductor.DeploymentStage    `json:"stage,omitempty"`
	Namespace             string                         `json:"namespace,omitempty"`
	NalejVariables        map[string]string              `json:"nalej_variables,omitempty"`
	ZtNetworkId           string                         `json:"zt_network_id,omitempty"`
	OrganizationId        string                         `json:"organization_id,omitempty"`
	OrganizationName      string                         `json:"organization_name,omitempty"`
	DeploymentId          string                         `json:"deployment_id,omitempty"`
	AppDescriptorId       string                         `json:"app_descriptor_id,omitempty"`
	AppDescriptorName     string                         `json:"app_descriptor_name,omitempty"`
	AppInstanceId         string                         `json:"app_instance_id,omitempty"`
	AppName               string                         `json:"app_name,omitempty"`
	ClusterPublicHostname string                         `json:"cluster_public_hostname,omitempty"`
	DNSHosts              []string                       `json:"dns_hosts,omitempty"`
	PublicCredentials     pbApplication.ImageCredentials `json:"public_credentials,omitempty"`
}

// EndpointType ---

type EndpointType int

const (
	ENDPOINT_TYPE_IS_ALIVE = iota
	ENDPOINT_TYPE_REST
	ENDPOINT_TYPE_WEB
	ENDPOINT_TYPE_PROMETHEUS
	ENDPOINT_TYPE_INGESTION
)

var EndpointTypeToGRPC = map[EndpointType]pbApplication.EndpointType{
	ENDPOINT_TYPE_IS_ALIVE:   pbApplication.EndpointType_IS_ALIVE,
	ENDPOINT_TYPE_REST:       pbApplication.EndpointType_REST,
	ENDPOINT_TYPE_WEB:        pbApplication.EndpointType_WEB,
	ENDPOINT_TYPE_PROMETHEUS: pbApplication.EndpointType_PROMETHEUS,
	ENDPOINT_TYPE_INGESTION:  pbApplication.EndpointType_INGESTION,
}

// EndpointInstance ---

type EndpointInstance struct {
	EndpointInstanceId string       `json:"endpoint_instance_id,omitempty"`
	EndpointType       EndpointType `json:"endpoint_type,omitempty"`
	FQDN               string       `json:"fqdn,omitempty"`
	Port               int32        `json:"port,omitempty"`
}

func (ei *EndpointInstance) ToGRPC() *pbApplication.EndpointInstance {
	return &pbApplication.EndpointInstance{
		EndpointInstanceId: ei.EndpointInstanceId,
		Type:               EndpointTypeToGRPC[ei.EndpointType],
		Fqdn:               ei.FQDN,
		Port:               ei.Port,
	}
}

func ValidateAuthorizeZTConnectionRequest(request *grpc_network_go.AuthorizeZTConnectionRequest) derrors.Error {
	if request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id cannot be empty")
	}
	if request.AppInstanceId == "" {
		return derrors.NewInvalidArgumentError("app_instance_id cannot be empty")
	}
	if request.MemberId == "" {
		return derrors.NewInvalidArgumentError("member_id cannot be empty")
	}
	if request.NetworkId == "" {
		return derrors.NewInvalidArgumentError("network_id cannot be empty")
	}
	return nil
}

func ValidateJoinZTNetworkRequest(request *grpc_deployment_manager_go.JoinZTNetworkRequest) derrors.Error {
	if request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id cannot be empty")
	}
	if request.AppInstanceId == "" {
		return derrors.NewInvalidArgumentError("app_instance_id cannot be empty")
	}
	if request.ServiceId == "" {
		return derrors.NewInvalidArgumentError("service_id cannot be empty")
	}
	if request.NetworkId == "" {
		return derrors.NewInvalidArgumentError("network_id cannot be empty")
	}
	return nil
}

func ValidateLeaveZTNetworkRequest(request *grpc_deployment_manager_go.LeaveZTNetworkRequest) derrors.Error {
	if request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id cannot be empty")
	}
	if request.AppInstanceId == "" {
		return derrors.NewInvalidArgumentError("app_instance_id cannot be empty")
	}
	if request.ServiceId == "" {
		return derrors.NewInvalidArgumentError("service_id cannot be empty")
	}
	if request.NetworkId == "" {
		return derrors.NewInvalidArgumentError("network_id cannot be empty")
	}
	return nil
}
