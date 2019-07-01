/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
)



// A executor is a middleware that transforms a deployment plan into an executable plan for the given
// platform. These translators are in charge of defining how the platform-specific entities have to be
// created/deployed in order to follow the deployment plan.
type Executor interface {

    // Get the namespace for this application. If any namespace is already available for the application
    // the name of this namespace must be returned. If not, a new valid namespace is returned.
    // params:
    //  organizationId
    //  appInstanceId
    //  numRetry
    // return:
    //  name for the namespace or error if any
    GetApplicationNamespace(organizationId string, appInstanceId string, numRetry int) (string, error)

    // Execute any initial preparation to deploy a fragment.
    //  params:
    //   data information for deployment
    //  return:
    //   error if any
    PrepareEnvironmentForDeployment(data entities.DeploymentMetadata) (Deployable, error)

    // Build a deployable object that can be executed into the current platform using its native description.
    //  params:
    //   data deployment metadata
    //  return:
    //   deployable entity or error if any
    BuildNativeDeployable(data entities.DeploymentMetadata) (Deployable, error)


    // Execute a deployment stage for the current platform.
    //  params:
    //   toDeploy items to be deployed
    //   fragment to the stage belongs to
    //   stage to be executed
    //  return:
    //   deployable object or error if any
    DeployStage(toDeploy Deployable, fragment *pbConductor.DeploymentFragment,stage *pbConductor.DeploymentStage,
        monitoredInstances monitor.MonitoredInstances) error

    // This operation should be executed after the failed deployment of a deployment stage. The target platform must
    // be ready to retry again the deployment of this stage. This means, that other deployable entities deployed
    // by other stages must be untouched.
    //  params:
    //   stage to be deployed
    //   toUndeploy deployable entities associated with the stage that have to be undeployed
    //  return:
    //   error if any
    UndeployStage(stage *pbConductor.DeploymentStage, toUndeploy Deployable) error

    // This operation undeploys a fragment from the system.
    //  params:
    //   namespace of the fragment
    //   fragmentId to be undeployed
    //  return:
    //   error if any
    UndeployFragment(namespace string, fragmentId string) error

    // This operation undeploys the namespace of an application
    //  params:
    //   request undeployment request
    //   toUndeploy deployable entities associated with the fragment that have to be undeployed
    //  return:
    //   error if any
    UndeployNamespace(request *pbDeploymentMgr.UndeployRequest) error

    // Generate an events controller for a given namespace.
    //  params:
    //   fragment to be supervised
    //   monitored data structure to monitor incoming events
    //   namespace to be observed
    //  return:
    //   deployment controller in charge of this namespace
    AddEventsController(fragmentId string, monitored monitor.MonitoredInstances, namespace string) DeploymentController

    // Start an event controller for the namespace.
    //  params:
    //   fragmentId to be supervised
    //  return:
    //   deployment controller in charge of this namespace
    StartControlEvents(fragmentId string) DeploymentController

    // Stop the control of events for a given namespace.
    //  params:
    //   fragmentId to stop supervising
    StopControlEvents(fragmentId string)
}

// A monitor system to inform the cluster API about the current status
type Monitor interface {

    // Update the status of the system including fragments and services. This function must send to conductor
    // the information corresponding to any service/fragment update in the system.
    UpdateStatus()

    // Run the service to periodically check pending updates.
    Run()
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
    // Add a monitor resource in the native platform using its uid and connect it with the corresponding service
    // and deployment stage.
    // params:
    //  resource
   AddMonitoredResource(resource *entities.MonitoredPlatformResource)

   // Sets the status of a resource in the system. The implementation is in charge of transforming the native
   // status value into a NalejServiceStatus
   // params:
   //  appInstanceID application
   //  serviceID service identifier
   //  uid native identifier
   //  status of the resource
   //  info relevant textual information
   //  endpoints for the resource
   SetResourceStatus(appInstanceID string, serviceID string, uid string, status entities.NalejServiceStatus, info string,
       endpoints []entities.EndpointInstance)

   // Start checking events
   Run()

   // Stop checking events
   Stop()
}