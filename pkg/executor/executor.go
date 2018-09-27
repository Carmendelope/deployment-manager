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
    //   error if any
    Execute(fragment *pbConductor.DeploymentFragment,stage *pbConductor.DeploymentStage) error

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
