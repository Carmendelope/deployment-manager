/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    "github.com/nalej/derrors"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/monitor"
)

// A executor is a middleware that transforms a deployment plan into an executable plan for the given
// platform. These translators are in charge of defining how the platform-specific entities have to be
// created/deployed in order to follow the deployment plan.
type Executor interface {


    // Execute any initial preparation to deploy a fragment.
    //  params:
    //   fragment to be deployed
    //   namespace the fragment belongs to
    //   monitor to overview the deployment
    //  return:
    //   error if any
    PrepareEnvironmentForDeployment(fragment *pbConductor.DeploymentFragment, namespace string,
        monitor *monitor.MonitorHelper) (Deployable, error)

    // Build a deployable object that can be executed into the current platform using its native description.
    //  params:
    //   stage to be deployed
    //   namespace where the stage has to be deployed
    //   ztNetworkId identifier for the zero-tier network these deployables will use
    //   organizationId identifier for the organization
    //   deploymentId identifier for the specific deployment
    //   appInstanceId identifier of the application instance
    //  return:
    //   deployable entity or error if any
    BuildNativeDeployable(stage *pbConductor.DeploymentStage, namespace string, ztNetworkId string,
        organizationId string, deploymentId string, appInstanceId string) (Deployable, error)

    // Execute a deployment stage for the current platform.
    //  params:
    //   toDeploy items to be deployed
    //   fragment to the stage belongs to
    //   stage to be executed
    //   monitor to inform about system information
    //  return:
    //   deployable object or error if any
    DeployStage(toDeploy Deployable, fragment *pbConductor.DeploymentFragment,stage *pbConductor.DeploymentStage,
        monitor *monitor.MonitorHelper) error

    // This operation should be executed after the failed deployment of a deployment stage. The target platform must
    // be ready to retry again the deployment of this stage. This means, that other deployable entities deployed
    // by other stages must be untouched.
    //  params:
    //   stage to be deployed
    //   toUndeploy deployable entities associated with the stage that have to be undeployed
    //  return:
    //   error if any
    UndeployStage(stage *pbConductor.DeploymentStage, toUndeploy Deployable) error

    // This operation should be executed after the failed deployment of a fragment. After running this operation,
    // any deployable associated with this fragment must be removed.
    //  params:
    //   fragment to be deployed
    //   toUndeploy deployable entities associated with the fragment that have to be undeployed
    //  return:
    //   error if any
    UndeployFragment(fragment *pbConductor.DeploymentStage, toUndeploy Deployable) error

    // This operation undeploys the namespace of an application
    //  params:
    //   request undeployment request
    //   toUndeploy deployable entities associated with the fragment that have to be undeployed
    //  return:
    //   error if any
    UndeployNamespace(request *pbConductor.UndeployRequest) derrors.Error

}


// This interface describes functions to be implemented by any deployable element that can be executed on top
// of an underlying platform.
type Deployable interface {
    // Get the unique identifier for this deployable.
    GetId() string
    // Build the deployable and construct the corresponding internal structures.
    Build() error
    // Deploy this element using a deployment controller to check when the operation is fully done.
    Deploy(controller DeploymentController) error
    // Undeploy this element
    Undeploy() derrors.Error
}

// Minimalistic interface to run a controller in charge of overviewing the successful deployment of requested
// operations. The system model must be updated accordingly.
type DeploymentController interface {
    // Add a monitor resource in the native platform using its uid and connect it with the corresponding service
    // and deployment stage.
    // params:
    //  uid native resource identifier
    //  serviceId nalej service identifier
    //  stageId for the deployment stage
   AddMonitoredResource(uid string, serviceId string, stageId string)

   // Sets the status of a resource in the system. The implementation is in charge of transforming the native
   // status value into a NalejServiceStatus
   // params:
   //  uid native identifier
   //  status of the resource
   SetResourceStatus(uid string, status entities.NalejServiceStatus)
}