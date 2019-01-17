/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package entities

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    "k8s.io/api/extensions/v1beta1"
)

// Service status definition

type NalejServiceStatus int

const (
    NALEJ_SERVICE_SCHEDULED     = iota
    NALEJ_SERVICE_WAITING
    NALEJ_SERVICE_DEPLOYING
    NALEJ_SERVICE_RUNNING
    NALEJ_SERVICE_ERROR
)

var ServiceStatusToGRPC = map[NalejServiceStatus] pbConductor.ServiceStatus {
    NALEJ_SERVICE_SCHEDULED: pbConductor.ServiceStatus_SERVICE_SCHEDULED,
    NALEJ_SERVICE_WAITING:   pbConductor.ServiceStatus_SERVICE_WAITING,
    NALEJ_SERVICE_DEPLOYING: pbConductor.ServiceStatus_SERVICE_DEPLOYING,
    NALEJ_SERVICE_RUNNING:   pbConductor.ServiceStatus_SERVICE_RUNNING,
    NALEJ_SERVICE_ERROR:     pbConductor.ServiceStatus_SERVICE_ERROR,
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
func KubernetesDeploymentStatusTranslation (kStatus v1beta1.DeploymentStatus) NalejServiceStatus {
    var result NalejServiceStatus
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
        result = NALEJ_SERVICE_ERROR
    } else if running == len(kStatus.Conditions) {
        result = NALEJ_SERVICE_RUNNING
    } else if progressing > 0 {
        result = NALEJ_SERVICE_DEPLOYING
    } else {
        result = NALEJ_SERVICE_WAITING
    }

    // log.Debug().Msgf("translate condition status %v into %s",kStatus.Conditions, result)

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
)

var FragmentStatusToGRPC = map[FragmentStatus] pbConductor.DeploymentFragmentStatus {
    FRAGMENT_WAITING : pbConductor.DeploymentFragmentStatus_WAITING,
    FRAGMENT_DEPLOYING : pbConductor.DeploymentFragmentStatus_DEPLOYING,
    FRAGMENT_DONE : pbConductor.DeploymentFragmentStatus_DONE,
    FRAGMENT_ERROR : pbConductor.DeploymentFragmentStatus_ERROR,
    FRAGMENT_RETRYING : pbConductor.DeploymentFragmentStatus_RETRYING,
}