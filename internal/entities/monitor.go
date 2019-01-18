/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package entities

import (
    "github.com/rs/zerolog/log"
)

// Set of entities used for monitoring services.

// Local entity to manage stage monitored entries. A stage contains several services.
type MonitoredStageEntry struct {
    // Stage identifier
    StageID string `json: "stage_id, omitempty"`
    // Organization Id these stages are running into
    OrganizationId string `json: "organization_id, omitempty"`
    // Instance Id these stages belong to
    InstanceId string `json: "instance_id, omitempty"`
    // fragment id for these stages
    FragmentId string `json: "fragment_id, omitempty"`
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
    // Organization Id these stages are running into
    OrganizationId string `json: "organization_id, omitempty"`
    // Instance Id these stages belong to
    InstanceId string `json: "instance_id, omitempty"`
    // fragment id for these stages
    FragmentId string `json: "fragment_id, omitempty"`
    // Service identifier
    ServiceID string `json: "service_id, omitempty"`
    // Number of pending checks to be done
    NumPendingChecks int `json: "num_pending_checks, omitempty"`
    // Map of resources being monitored for this Nalej service.
    // UID -> resource data
    Resources map[string]MonitoredPlatformResource `json: "resources, omitempty"`
    // Resource status
    Status NalejServiceStatus `json: "status, omitempty"`
    // Flag indicating if a new status has been set without notification
    NewStatus bool `json: "new_status, omitempty"`
    // Relevant textual information
    Info string `json: "info, omitempty"`
    // Endpoints serving data
    Endpoints [] string
}

// Add a pending resource to a monitored Nalej service entry
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
    Status NalejServiceStatus `json: "status, omitempty"`
}

func NewMonitoredPlatformResource(uid string, stageID string, serviceID string, info string) MonitoredPlatformResource {
    return MonitoredPlatformResource{
        UID: uid, StageID: stageID, Info: info, Status:NALEJ_SERVICE_DEPLOYING, ServiceID: serviceID}

}
