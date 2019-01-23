/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package entities

import "github.com/rs/zerolog/log"

// Set of entities used for monitoring services.


// Local entity to manage a monitored app. This structure simply indicates what nalej services are running.
type MonitoredAppEntry struct {
    // Organization Id these stages are running into
    OrganizationId string `json: "organization_id, omitempty"`
    // Instance Id these stages belong to
    InstanceId string `json: "instance_id, omitempty"`
    // Deployment app id
    DeploymentId string `json: "deployment_id, omitempty"`
    // Status
    Status FragmentStatus `json: "status, omitempty"`
    // fragment id for these stages
    FragmentId string `json: "fragment_id, omitempty"`
    // Nalej Services
    // ServiceID -> UID -> Resource
    Services map[string]*MonitoredServiceEntry `json: "services, omitempty"`
    // Number of pending checks to be done
    NumPendingChecks int `json: "num_pending_checks, omitempty"`
    // Additional info
    Info string `json: "num_pending_checks, omitempty"`
    // Total services
    TotalServices int `json: "total_services, omitempty"`
}

// Add a new application entry. If the entry already exists, it adds the new services.
func(m *MonitoredAppEntry) AppendServices(new *MonitoredAppEntry) {
    for _, serv := range new.Services {
        _, found := m.Services[serv.ServiceID]
        if !found {
            // This is new. Add it
            m.Services[serv.ServiceID] = serv
            m.NumPendingChecks = m.NumPendingChecks + 1
        }
    }
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
    Resources map[string]*MonitoredPlatformResource `json: "resources, omitempty"`
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
func (m *MonitoredServiceEntry) AddPendingResource(toAdd *MonitoredPlatformResource) {
    _, found := m.Resources[toAdd.UID]
    if found {
        log.Debug().Interface("resource", toAdd).Msg("resource already exists")
        return
    }
    toAdd.Pending = true
    m.Resources[toAdd.UID] = toAdd
    m.NumPendingChecks = m.NumPendingChecks + 1
}

// Set a resource as no longer pending.
func (m *MonitoredServiceEntry) RemovePendingResource(uid string) {
    res, found := m.Resources[uid]
    if !found {
        log.Error().Str("serviceId", m.ServiceID).Str("uid",uid).
            Msg("cannot remove pending resource from monitored service entry. Not found")
        return
    }

    res.Pending = false
    m.NumPendingChecks = m.NumPendingChecks - 1
    log.Debug().Str("serviceId", m.ServiceID).Str("uid",uid).Int("numPendingChecks", m.NumPendingChecks).
        Msg("the number of pending checks was modified")
}


type MonitoredPlatformResource struct {
    // Stage this resource belongs to
    AppInstanceID string `json: "app_instance_id, omitempty"`
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

func NewMonitoredPlatformResource(uid string, appInstanceId string, serviceID string, info string) MonitoredPlatformResource {
    return MonitoredPlatformResource{
        UID: uid, AppInstanceID: appInstanceId, Info: info, Status:NALEJ_SERVICE_SCHEDULED, ServiceID: serviceID, Pending: true}

}
