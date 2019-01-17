/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/rs/zerolog/log"
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

    // Add a new resource pending to be checked.
    // params:
    //  newResource to be checked
    AddPendingResource(newResource MonitoredPlatformResource)

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

}


// Local entity to manage stage monitored entries. A stage contains several services.
type MonitoredStageEntry struct {
    // Organization Id these stages are running into
    OrganizationId string `json: "organization_id, omitempty"`
    // Instance Id these stages belong to
    InstanceId string `json: "instance_id, omitempty"`
    // fragment id for these stages
    FragmentId string `json: "fragment_id, omitempty"`
    // Main deployable object
    ToDeploy executor.Deployable
    // Nalej Services
    // ServiceID -> UID -> Resource
    Services map[string]MonitoredServiceEntry `json: "services, omitempty"`
    // Number of pending checks to be done
    NumPendingChecks int `json: "num_pending_checks, omitempty"`
}


// Add a pending resource to a stage entry
func (m *MonitoredStageEntry) AddPendingResource(toAdd MonitoredPlatformResource) {
    service, found := m.Services[toAdd.ServiceID]
    if !found {
        log.Error().Str("serviceID",toAdd.ServiceID).Msg("unknown monitored service")
    }
    service.AddPendingResource(toAdd)
    m.NumPendingChecks = m.NumPendingChecks + 1
}


type MonitoredServiceEntry struct {
    // Service identifier
    ServiceID string `json: "service_id, omitempty"`
    // Number of pending checks to be done
    NumPendingChecks int `json: "num_pending_checks, omitempty"`
    // Map of resources being monitored for this Nalej service.
    // UID -> resource data
    Resources map[string]MonitoredPlatformResource `json: "resources, omitempty"`
    // Resource status
    Status entities.NalejServiceStatus `json: "status, omitempty"`
}

func (m *MonitoredServiceEntry) AddPendingResource(toAdd MonitoredPlatformResource) {
    toAdd.Pending = true
    m.Resources[toAdd.UID] = toAdd
    m.NumPendingChecks = m.NumPendingChecks + 1
}


type MonitoredPlatformResource struct {
    // Stage this resource belongs to
    StageID string `json: "stage, omitempty"`
    // Service ID this resource belongs to
    ServiceID string `json: "service_id, omitempty"`
    // Local UID used for this resource
    UID string `json: "uid, omitempty"`
    // Textual information
    Info string `json: "info, omitempty"`
    // Flag indicating whether this resource is pending or not
    Pending bool `json: "pending, omitempty"`
    // Resource status
    Status entities.NalejServiceStatus `json: "status, omitempty"`
}

func NewMonitoredPlatformResource(uid string, stageID, info string) MonitoredPlatformResource {
    return MonitoredPlatformResource{
        UID: uid, StageID: stageID, Info: info,
    }
}
