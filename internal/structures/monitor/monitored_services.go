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
    // Check iteratively if the app has any pending resource to be deployed. This is done using the kubernetes controller.
    // If after the maximum expiration time the check is not successful, the execution is considered to be failed.
    // params:
    //  appInstanceId
    //  timecheck seconds between checks
    //  timeout seconds to wait until considering the task to be failed
    // return:
    //  error if any
    WaitPendingChecks(appInstanceId string, timecheck int, timeout int) error

    // Add a new app to be monitored. If the application already exists, the services are added to the current instance.
    // params:
    //  toAdd application to be added.
    AddApp(toAdd *entities.MonitoredAppEntry)

    // Set the status of a fragment
    // params:
    //  appInstanceId
    //  status
    //  err execution error
    SetAppStatus(appInstanceId string, status entities.FragmentStatus, err error)

    // Add a new resource pending to be checked.
    // params:
    //  newResource to be checked
    AddPendingResource(newResource *entities.MonitoredPlatformResource) bool

    // Remove a resource from the list.
    // params:
    //  uid internal platform identifier
    // returns:
    //  false if not found
    RemovePendingResource(stageID string, serviceInstanceID string, uid string) bool

    // Check if a platform resource is monitored
    // params:
    //  uid internal platform identifier
    // returns:
    //  true if the resource is monitored
    IsMonitoredResource(stageID string, serviceInstanceID string, uid string) bool


    // Set the status of a resource. This function determines how to change the service status
    // depending on the combination of the statuses of its related resources.
    // params:
    //  appInstanceId stage identifier
    //  uid native resource identifier
    //  status of the native resource
    //  info textual information if proceeds
    //  endpoints optional array of endpoints
    SetResourceStatus(appInstanceId string, serviceInstanceId string, uid string, status entities.NalejServiceStatus, info string,
        endpoints []entities.EndpointInstance)

    // This function returns a list of monitored apps with pending notifications and their services with pending notifications.
    // returns:
    //  array with the collection of monitored apps with pending notifications
    GetPendingNotifications() ([] *entities.MonitoredAppEntry)

    // Set to already notified all services.
    ResetServicesUnnotifiedStatus()

    // Check the status of the services and set the app status and update entries accordingly.
    //  params:
    //   appInstanceID identifier of the application
    UpdateAppStatus(appInstanceID string)

    // Remove an existing app
    // params:
    //  appInstanceId app to be removed
    // return:
    //  true if the app was deleted
    RemoveApp(appInstanceId string) bool

    // Return the total number of monitored applications
    // return:
    //  number of monitored applications
    GetNumApps() int

    // Return the total number of monitored services
    // return:
    //  number of monitored services
    GetNumServices() int

    // Return the total number of monitored resources
    // return:
    //  number of monitored resources
    GetNumResources() int

}


