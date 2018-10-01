/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbApplication "github.com/nalej/grpc-application-go"
)

// A executor is a middleware that transforms a deployment plan into an executable plan for the given
// platform. These translators are in charge of defining how the platform-specific entities have to be
// created/deployed in order to follow the deployment plan.
type Executor interface {

    // Execute a deployment stage for the current platform.
    //  params:
    //   fragment to the stage belongs to
    //   stage to be executed
    //  return:
    //   deployable object or error if any
    Execute(fragment *pbConductor.DeploymentFragment,stage *pbConductor.DeploymentStage) (*Deployable,error)

    // Operation to be run in case a stage deployment fails. The rollback should bring the system to
    // the system status before this stage was executed.
    //  params:
    //   plan to be executed
    //   lastDeployed last stage that was deployed
    //  return:
    //   error if any
    StageRollback(stage *pbConductor.DeploymentStage) error

    // Undeploy a service
    //  params:
    //   serv the service to be undeployed
    //  return:
    //
    UndeployService(serv *pbApplication.Service) error
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
    Undeploy() error
}

// Minimalistic interface to run a controller in charge of overviewing the successful deployment of requested
// operations.
type DeploymentController interface {
    // Add a resource to be monitored indicating its id on the target platform (uid) and the stage identifier.
   AddMonitoredResource(uid string, stageId string)
}