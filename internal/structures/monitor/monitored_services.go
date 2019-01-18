/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
)

// Structure designed to observe the evolution of ongoing deployed/deployments in the current cluster.
// This structure can be used to inform other solutions about the current deployment.

type MonitoredInstances interface {
    // Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
    // If after the maximum expiration time the check is not successful, the execution is considered to be failed.
    // params:
    //  stageId
    //  timecheck seconds between checks
    //  timeout seconds to wait until considering the task to be failed
    // return:
    //  error if any
    WaitPendingChecks(stageId string, timecheck int, timeout int) error

    // Add the information for a stage to be monitored
    //  params:
    //   toAdd entry to be added
    AddStage(toAdd entities.MonitoredStageEntry)

    // Add a new resource pending to be checked.
    // params:
    //  newResource to be checked
    AddPendingResource(newResource entities.MonitoredPlatformResource)

    // Remove a resource from the list.
    // params:
    //  uid internal platform identifier
    // returns:
    //  false if not found
    RemovePendingResource(uid string) bool

    // Check if a platform resource is monitored
    // params:
    //  uid internal platform identifier
    // returns:
    //  true if the resource is monitored
    IsMonitoredResource(uid string) bool

    // Check if a stage has pending checks
    // params:
    //  stageID
    // returns:
    //  true if there are pending entries
    StageHasPendingChecks(stageID string) bool

    // Set the status of a resource. This function determines how to change the service status
    // depending on the combination of the statuses of its related resources.
    // params:
    //  uid native resource identifier
    //  status of the native resource
    //  endpoints optional array of endpoints
    //  info textual information if proceeds
    SetResourceStatus(uid string, status entities.NalejServiceStatus, info string)

    // Return the list of services with a new status to be notified.
    // returns:
    //  array with the collection of entities with a service with a status pending of notification
    GetServicesUnnotifiedStatus() [] entities.MonitoredServiceEntry

    // Set to already notified all services.
    ResetServicesUnnotifiedStatus()

}


