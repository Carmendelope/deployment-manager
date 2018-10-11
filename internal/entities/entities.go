package entities

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    "k8s.io/api/extensions/v1beta1"
    "github.com/rs/zerolog/log"
)

// Service status definition

type ServiceStatus int

const (
    SERVICE_SCHEDULED = iota
    SERVICE_WAITING
    SERVICE_DEPLOYING
    SERVICE_RUNNING
    SERVICE_ERROR
)

var ServiceStatusToGRPC = map[ServiceStatus] pbConductor.ServiceStatus {
    SERVICE_SCHEDULED : pbConductor.ServiceStatus_SERVICE_SCHEDULED,
    SERVICE_WAITING : pbConductor.ServiceStatus_SERVICE_WAITING,
    SERVICE_DEPLOYING : pbConductor.ServiceStatus_SERVICE_DEPLOYING,
    SERVICE_RUNNING : pbConductor.ServiceStatus_SERVICE_RUNNING,
    SERVICE_ERROR : pbConductor.ServiceStatus_SERVICE_ERROR,
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
func KubernetesDeploymentStatusTranslation (kStatus v1beta1.DeploymentStatus) ServiceStatus {
    var result ServiceStatus
    running := 0
    progressing := 0
    error := 0
    for _, c := range kStatus.Conditions {
        switch c.Type {
        case v1beta1.DeploymentAvailable:
            running = running + 1
        case v1beta1.DeploymentProgressing:
            progressing  = progressing + 1
        default:
            // this is a failure
            error = error + 1
        }
    }
    if error > 0 {
        result = SERVICE_ERROR
    } else if running == len(kStatus.Conditions) {
        result = SERVICE_RUNNING
    } else if progressing > 0 {
        result = SERVICE_DEPLOYING
    } else {
        result = SERVICE_WAITING
    }


    log.Debug().Msgf("translate condition status %v into %s",kStatus.Conditions, result)

    return result
}




// Deployment fragment status definition

type FragmentStatus int

const (
    FRAGMENT_WAITING = iota
    FRAGMENT_DEPLOYING
    FRAGMENT_DONE
    FRAGMENT_ERROR
    FRAGMENT_RETRYING
)

var FragmentStatusToGRPC = map[FragmentStatus] pbConductor.DeploymentFragmentStatus {
    FRAGMENT_WAITING : pbConductor.DeploymentFragmentStatus_WAITING,
    FRAGMENT_DEPLOYING : pbConductor.DeploymentFragmentStatus_DEPLOYING,
    FRAGMENT_DONE : pbConductor.DeploymentFragmentStatus_DONE,
    FRAGMENT_ERROR : pbConductor.DeploymentFragmentStatus_ERROR,
    FRAGMENT_RETRYING : pbConductor.DeploymentFragmentStatus_RETRYING,
}