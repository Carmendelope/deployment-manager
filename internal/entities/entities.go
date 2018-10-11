package entities

import pbConductor "github.com/nalej/grpc-conductor-go"

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