/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/nalej/deployment-manager/pkg/monitor"
)

// A executor is a middleware that transforms a deployment plan into an executable plan for the given
// platform. These translators are in charge of defining how the platform-specific entities have to be
// created/deployed in order to follow the deployment plan.
type Executor interface {

    // Execute a deployment stage for the current platform.
    //  params:
    //   fragment to the stage belongs to
    //   stage to be executed
    //   monitor to inform about system information
    //  return:
    //   deployable object or error if any
    Execute(fragment *pbConductor.DeploymentFragment,stage *pbConductor.DeploymentStage,
        monitor *monitor.MonitorHelper) (*Deployable,error)

    // Operation to be run in case a stage deployment fails. The rollback should bring the system to
    // the system status before this stage was executed.
    //  params:
    //   stage this rollback belongs to
    //   deployed entries
    //  return:
    //   error if any
    StageRollback(stage *pbConductor.DeploymentStage, deployed Deployable) error

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
// operations. The system model must be updated accordingly.
type DeploymentController interface {
   AddMonitoredResource(uid string, stageId string)
}