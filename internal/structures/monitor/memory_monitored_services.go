/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package monitor

// In memory implementation of a monitored services control structure.

import (
    "errors"
    "fmt"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/rs/zerolog/log"
    "sync"
    "time"
)


// We store the number of pending checks for a certain stage. Every time a check is done, we reduce the number
// of pending stages until, it's zero. For every pending resource we store an inverted pointer to the stage it
// belongs to.
type MemoryMonitoredInstances struct {
    // Monitored resources for a given stage
    // app instance id -> entry
    monitoredApps map[string]*entities.MonitoredAppEntry
    // Mutex
    mu sync.RWMutex
}

// Constructor to instantiate a basic memory monitored instances object.
func NewMemoryMonitoredInstances() MonitoredInstances {
    return &MemoryMonitoredInstances{
        monitoredApps: make(map[string]*entities.MonitoredAppEntry,0),
    }
}

func (p *MemoryMonitoredInstances) AddApp(toAdd *entities.MonitoredAppEntry) {
    p.mu.Lock()
    defer p.mu.Unlock()
    current, found := p.monitoredApps[toAdd.AppInstanceId]
    if !found {
        // new entry
        log.Debug().Str("instanceId",toAdd.AppInstanceId).Msg("new app to be monitorized")
        p.monitoredApps[toAdd.AppInstanceId] = toAdd
    } else {
        // Add new services if they were not previously added
        log.Debug().Str("instanceId",toAdd.AppInstanceId).Msg("append new services to app")
        current.AppendServices(toAdd)
    }
}

func(p *MemoryMonitoredInstances) SetAppStatus(appInstanceId string, status entities.FragmentStatus, err error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    current, found := p.monitoredApps[appInstanceId]
    if !found {
        log.Debug().Str("instanceId",appInstanceId).Msg("impossible to set status. No monitored app")
    } else {
        // Add new services if they were not previously added
        log.Debug().Str("instanceId",appInstanceId).Interface("status",status).Msg("set instance status")
        current.Status = status
        if err !=  nil {
            current.Info = err.Error()
        } else {
            current.Info = ""
        }
    }
}


// Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
// If after the maximum expiration time the check is not successful, the execution is considered to be failed.
func(p *MemoryMonitoredInstances) WaitPendingChecks(appInstanceId string, checkingSleepTime int, stageCheckingTimeout int) error {
    log.Info().Msgf("app %s wait until services for the instance are ready",appInstanceId)
    timeout := time.After(time.Second * time.Duration(stageCheckingTimeout))
    tick := time.Tick(time.Second * time.Duration(checkingSleepTime))
    for {
        select {
        // Got a timeout! Error
        case <-timeout:
            log.Error().Str("instanceId",appInstanceId).Msg("checking pendingStages resources exceeded for stage")
            return errors.New(fmt.Sprintf("checking pendingStages resources exceeded for app %s", appInstanceId))
            // Next check
        case <-tick:
            p.UpdateAppStatus(appInstanceId)
            monitoredEntry, found := p.monitoredApps[appInstanceId]
            if !found {
                log.Info().Str("appInstanceID", appInstanceId).Msg("app not monitored")
                return errors.New(fmt.Sprintf("not monitored app %s", appInstanceId))
            }
            if monitoredEntry.NumPendingChecks == 0 {
                log.Info().Str("instanceId",appInstanceId).Msg("app has no pendingStages checks. Exit checking stage")
                return nil
            }
        }
    }
}



// Add a new resource pending to be checked.
func(p *MemoryMonitoredInstances) AddPendingResource(newResource *entities.MonitoredPlatformResource) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    log.Debug().Interface("newResource", newResource).Msg("add new pending resource")

    appEntry, found := p.monitoredApps[newResource.AppInstanceID]
    if !found {
        log.Error().Str("appInstanceID", newResource.AppInstanceID).Msg("impossible to add resource. App not monitored.")
        return false
    }

    // Get the service
    service, found := appEntry.Services[newResource.ServiceInstanceID]
    if !found {
        log.Error().Str("appInstanceID", newResource.AppInstanceID).Str("serviceInstanceID", newResource.ServiceInstanceID).
            Msg("impossible to add resource. Service not monitored.")
        return false
    }
    service.AddPendingResource(newResource)

    log.Debug().Str("appInstanceID", newResource.AppInstanceID).Int("pending checks",service.NumPendingChecks).
        Msg("a new resource has been added")
    return true
}

func(p *MemoryMonitoredInstances) RemovePendingResource(appInstanceID string, serviceInstanceID string, uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    appEntry, found := p.monitoredApps[appInstanceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Msg("impossible to remove resource. App not monitored.")
        return false
    }

    // Get the service
    service, found := appEntry.Services[serviceInstanceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Str("serviceInstanceID", serviceInstanceID).
            Msg("impossible to remove resource. Service not monitored.")
        return false
    }

    // -> resource
    pendingResource, found := service.Resources[uid]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Str("serviceInstanceID", serviceInstanceID).Str("resource uid", uid).
            Msg("impossible to remove resource. Resource not monitored")
        return false
    }

    // This is not pending
    pendingResource.Pending = false
    // One less check to fulfill
    service.NumPendingChecks = service.NumPendingChecks -1
    if service.NumPendingChecks == 0 {
        appEntry.NumPendingChecks = appEntry.NumPendingChecks - 1
    }

    return true
}


// Return true if the passed uid corresponds to a resource being monitored.
func (p *MemoryMonitoredInstances) IsMonitoredResource(appInstanceID string, serviceInstanceID string, uid string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    appEntry, found := p.monitoredApps[appInstanceID]
    if !found {
        return false
    }

    // Get the service
    service, found := appEntry.Services[serviceInstanceID]
    if !found {
        return false
    }

    // -> resource
    _, found = service.Resources[uid]
    if !found {
        return false
    }

    return true
}



func (p *MemoryMonitoredInstances) SetResourceStatus(appInstanceID string, serviceInstanceId, uid string,
    status entities.NalejServiceStatus, info string, endpoints []entities.EndpointInstance) {
    p.mu.Lock()
    defer p.mu.Unlock()

    log.Debug().Str("appInstanceID", appInstanceID).Str("serviceInstanceID", serviceInstanceId).Str("uid",uid).
        Interface("status",status).Str("info",info).Msg("set resource status")

    app, found := p.monitoredApps[appInstanceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Msg("impossible to set resource status. App not monitored.")
        return
    }

    // Get the service
    service, found := app.Services[serviceInstanceId]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Str("serviceInstanceID", serviceInstanceId).
            Interface("monitored",p.monitoredApps).Msg("impossible to set resource. Service not monitored.")
        return
    }

    // -> resource
    resource, found := service.Resources[uid]
    if !found {
        log.Warn().Str("appInstanceID", appInstanceID).Str("seviceInstanceId", serviceInstanceId).Str("resource uid", uid).
            Msg("resource was not added before setting a new status. We add it now")
        metadata := entities.DeploymentMetadata{
            AppDescriptorId: service.AppDescriptorId,
            AppInstanceId: service.AppInstanceId,
            ServiceGroupId: service.ServiceGroupId,
            ServiceGroupInstanceId: service.ServiceGroupInstanceId,
        }
        newResource := entities.NewMonitoredPlatformResource(uid, metadata, appInstanceID,serviceInstanceId, info)
        service.AddPendingResource(&newResource)
        resource = service.Resources[uid]
    }

    // If we are going to set the same status, exit.
    if resource.Status == status {
        log.Debug().Str("appInstanceID", appInstanceID).Str("serviceInstanceId", serviceInstanceId).Str("uid",uid).
            Interface("status",status).Str("info",info).Msg("no resource status changed")
        return
    }

    // Modify the status
    resource.Status = status
    resource.Info = info

    // set the endpoints for this entry
    service.Endpoints = endpoints
    /*
    if len(endpoints) >0 {
        if service.Endpoints == nil {
            service.Endpoints = endpoints
        } else {
            // add the endpoint if it is new
            found := false
            for _, ep := range service.Endpoints {
                if ep == endpoint {
                    // It is already there, exit
                    found = true
                    break
                }
            }
            if !found {
                service.Endpoints = append(service.Endpoints, endpoint)
            }
        }
    }
    */

    // If this is running remove one check
    if resource.Status == entities.NALEJ_SERVICE_RUNNING {
        service.RemovePendingResource(resource.UID)
    }

    // Update service status
    // get the worst status found in the resources required by this service
    previousStatus := service.Status
    var finalStatus entities.NalejServiceStatus
    finalStatus = entities.NALEJ_SERVICE_ERROR
    newServiceInfo := service.Info
    for _,res := range service.Resources {
        if res.Status == entities.NALEJ_SERVICE_ERROR {
            finalStatus = entities.NALEJ_SERVICE_ERROR
            newServiceInfo = res.Info
            break
        } else if res.Status < finalStatus {
            finalStatus = res.Status
            info = res.Info
        }
    }
    if finalStatus != previousStatus {
        log.Debug().Str("serviceInstanceID", service.ServiceInstanceID).Interface("status",finalStatus).Msg("service changed status")
        service.NewStatus = true
    }

    service.Status = finalStatus
    service.Info = newServiceInfo

    // update app status
    // the update will be only done if all the services are under deployed
    // get the worst status found in the services
    if len(app.Services) != app.TotalServices {
        return
    }


    var newAppStatus entities.NalejServiceStatus
    newAppStatus = entities.NALEJ_SERVICE_ERROR
    newAppInfo := app.Info
    for _, serv := range app.Services {
        if serv.Status == entities.NALEJ_SERVICE_ERROR {
            newAppStatus = entities.NALEJ_SERVICE_ERROR
            newAppInfo = serv.Info
            break
        } else if serv.Status < newAppStatus {
            newAppStatus = serv.Status
            newAppInfo = serv.Info
        }
    }

    app.Status = entities.ServicesToFragmentStatus[newAppStatus]
    app.Info = newAppInfo
}

func (p *MemoryMonitoredInstances) GetPendingNotifications() ([] *entities.MonitoredAppEntry) {
    p.mu.RLock()
    p.mu.RUnlock()
    toReturn := make([]*entities.MonitoredAppEntry,0)
    for _, app := range p.monitoredApps {
        pendingServices := make(map[string]*entities.MonitoredServiceEntry,0)
        // for every monitored app
        for _, x := range app.Services {
            // for every monitored service
            if x.NewStatus {
                pendingServices[x.ServiceInstanceID] = x
            }
        }
        if len(pendingServices) > 0 {
            newApp := entities.MonitoredAppEntry{
                FragmentId:       app.FragmentId,
                NumPendingChecks: app.NumPendingChecks,
                OrganizationId:   app.OrganizationId,
                AppInstanceId:    app.AppInstanceId,
                Services:         pendingServices,
                Info:             app.Info,
                DeploymentId:     app.DeploymentId,
                Status:           app.Status,
                AppDescriptorId:  app.AppDescriptorId,
            }
            toReturn = append(toReturn, &newApp)
        }
    }
    return toReturn
}

// Return the list of services with a new status to be notified.
// returns:
//  array with the collection of entities with a service with a status pending of notification
func (p *MemoryMonitoredInstances) GetServicesUnnotifiedStatus() [] *entities.MonitoredServiceEntry {
    p.mu.RLock()
    p.mu.RUnlock()
    toNotify := make([]*entities.MonitoredServiceEntry,0)
    for _, app := range p.monitoredApps {
        // for every monitored app
        for _, x := range app.Services {
            // for every monitored service
            if x.NewStatus {
                toNotify = append(toNotify, x)
            }
        }
    }
    return toNotify
}

// Set to already notified all services.
func (p *MemoryMonitoredInstances) ResetServicesUnnotifiedStatus() {
    p.mu.Lock()
    p.mu.Unlock()
    for _, stage := range p.monitoredApps {
        // for every monitored stage
        for _, x := range stage.Services {
            // for every monitored service
            if x.NewStatus {
                x.NewStatus = false
            }
        }
    }
    // log.Debug().Interface("monitored stages",p.monitoredStages).Msg("monitored after reset")
}

func (p *MemoryMonitoredInstances) UpdateAppStatus(appInstanceID string) {
    app, found := p.monitoredApps[appInstanceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Msg("impossible to update app status. App not monitored.")
        return
    }
    pendingServices := 0
    for _, serv := range app.Services {
        if serv.Status != entities.NALEJ_SERVICE_RUNNING {
            pendingServices = pendingServices + 1
        }
    }
    if app.NumPendingChecks != pendingServices {
        app.NumPendingChecks = pendingServices
        log.Info().Str("appInstanceID", appInstanceID).Int("pendingServices",app.NumPendingChecks).
            Msg("updated number of pending services for app")
    }
}


func (p *MemoryMonitoredInstances)  RemoveApp(appInstanceId string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    _, found := p.monitoredApps[appInstanceId]
    if !found {
        log.Error().Str("appInstanceId", appInstanceId).Msg("impossible to delete monitored entry. App not found")
        return false
    }

    delete(p.monitoredApps, appInstanceId)
    return true
}

// Return true if the passed uid corresponds to a resource being monitored.
/*
func (p *MemoryMonitoredInstances) getResource (appInstanceID string, serviceID, uid string) *entities.MonitoredPlatformResource {
    p.mu.RLock()
    defer p.mu.RUnlock()
    appEntry, found := p.monitoredApps[appInstanceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Msg("impossible to get resource. App not monitored.")
        return nil
    }

    // Get the service
    service, found := appEntry.Services[serviceID]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Str("stageID", serviceID).
            Msg("impossible to get resource. Service not monitored.")
        return nil
    }

    // -> resource
    pendingResource, found := service.Resources[uid]
    if !found {
        log.Error().Str("appInstanceID", appInstanceID).Str("stageID", serviceID).Str("resource uid", uid).
            Msg("impossible to get resource. Resource not monitored")
        return nil
    }

    return pendingResource
}
*/