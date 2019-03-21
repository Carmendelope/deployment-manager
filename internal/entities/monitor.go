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
    // App Descriptor id
    AppDescriptorId string `json: "app_descriptor_id, omitempty"`
    // Instance Id these stages belong to
    AppInstanceId string `json: "app_instance_id, omitempty"`
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
        _, found := m.Services[serv.ServiceInstanceID]
        if !found {
            // This is new. Add it
            m.Services[serv.ServiceInstanceID] = serv
            m.NumPendingChecks = m.NumPendingChecks + 1
        }
    }
}

type MonitoredServiceEntry struct {
    // Organization Id these stages are running into
    OrganizationId string `json: "organization_id, omitempty"`
    // App Descriptor Id these stages belong to
    AppDescriptorId string `json: "app_descriptor_id, omitempty"`
    // AppInstanceId
    AppInstanceId string `json: "app_instance_id, omitempty"`
    // Service group
    ServiceGroupId string `json: "service_group_id, omitempty"`
    // Service group instance id
    ServiceGroupInstanceId string `json: "service_group_instance_id, omitempty"`
    // fragment id for these stages
    FragmentId string `json: "fragment_id, omitempty"`
    // Service identifier
    ServiceID string `json: "service_id, omitempty"`
    //ServiceName
    ServiceName string `json: "service_name, omitempty"`
    // Service instance identifier
    ServiceInstanceID string `json: "service_instance_id, omitempty"`
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
    Endpoints [] EndpointInstance
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
        log.Error().Str("serviceInstanceId", m.ServiceInstanceID).Str("uid",uid).
            Msg("cannot remove pending resource from monitored service entry. Not found")
        return
    }

    res.Pending = false
    m.NumPendingChecks = m.NumPendingChecks - 1
    log.Debug().Str("serviceInstanceId", m.ServiceInstanceID).Str("uid",uid).Int("numPendingChecks", m.NumPendingChecks).
        Msg("the number of pending checks was modified")
}


type MonitoredPlatformResource struct {
    // AppID for this resource
    AppID string `json: "app_id, omitempty"`
    // Stage this resource belongs to
    AppInstanceID string `json: "app_instance_id, omitempty"`
    // Service Group this resource belongs to
    ServiceGroupID string `json: "service_group_id, omitempty"`
    // Service Group instance this resource belongs to
    ServiceGroupInstanceID string `json: "service_group_instance_id, omitempty"`
    // Service ID this resource belongs to
    ServiceID string `json: "service_id, omitempty"`
    // Service instance ID for this resource
    ServiceInstanceID string `json: "service_instance_id, omitempty"`
    // Local UID used for this resource
    UID string `json: "uid, omitempty"`
    // Textual information
    Info string `json: "info, omitempty"`
    // Flag indicating whether this resource is pending or not
    Pending bool `json: "pending, omitempty"`
    // Resource status
    Status NalejServiceStatus `json: "status, omitempty"`
}



func NewMonitoredPlatformResource(uid string, appDescriptorId string, appInstanceId string, serviceGroupId string,
    serviceGroupInstanceId string, serviceId string, serviceInstanceId string, info string) MonitoredPlatformResource {
    return MonitoredPlatformResource{
        UID: uid,
        AppID: appDescriptorId,
        AppInstanceID: appInstanceId,
        ServiceGroupID: serviceGroupId,
        ServiceGroupInstanceID: serviceGroupInstanceId,
        ServiceID: serviceId,
        ServiceInstanceID: serviceInstanceId,
        Info: info, Status:NALEJ_SERVICE_SCHEDULED, Pending: true}
}
