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
    // fragment id -> entry
    monitoredEntries map[string]*entities.MonitoredAppEntry
    // Mutex
    mu sync.RWMutex
}

// Constructor to instantiate a basic memory monitored instances object.
func NewMemoryMonitoredInstances() MonitoredInstances {
    return &MemoryMonitoredInstances{
        monitoredEntries: make(map[string]*entities.MonitoredAppEntry,0),
    }
}

func (p *MemoryMonitoredInstances) AddEntry(toAdd *entities.MonitoredAppEntry) {
    p.mu.Lock()
    defer p.mu.Unlock()
    current, found := p.monitoredEntries[toAdd.FragmentId]
    if !found {
        // new entry
        log.Debug().Str("fragmentId",toAdd.FragmentId).Msg("new fragment to be monitorized")
        p.monitoredEntries[toAdd.FragmentId] = toAdd
    } else {
        // Add new services if they were not previously added
        log.Debug().Str("fragmentId",toAdd.FragmentId).Msg("append new services to fragment")
        current.AppendServices(toAdd)
    }
}

func(p *MemoryMonitoredInstances) GetEntry(fragmentId string) *entities.MonitoredAppEntry {
    p.mu.Lock()
    defer p.mu.Unlock()
    current, found := p.monitoredEntries[fragmentId]
    if !found {
        return nil
    }
    return current
}

func(p *MemoryMonitoredInstances) SetEntryStatus(fragmentId string, status entities.FragmentStatus, err error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    current, found := p.monitoredEntries[fragmentId]
    if !found {
        log.Debug().Str("fragmentId",fragmentId).Msg("impossible to set status. No monitored app")
    } else {
        // Add new services if they were not previously added
        log.Debug().Str("fragmentId",fragmentId).Interface("status",status).Msg("set instance status")
        if status != current.Status {
            current.NewStatus = true
        }
        current.Status = status
        if err !=  nil {
            current.Info = err.Error()
        } else {
            current.Info = ""
        }
    }
}

func (p *MemoryMonitoredInstances) SetAppStatus(appInstanceId string, status entities.FragmentStatus, err error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    // iterate and update the status of the entries
    for _, current := range p.monitoredEntries {
        if current.AppInstanceId == appInstanceId {
            if status != current.Status {
                current.NewStatus = true
                current.Status = status
                // all services are in the same status
                for i, _ := range current.Services {
                    newStatus := entities.FragmentStatusToNalejServiceStatus[status]
                    if current.Services[i].Status != newStatus {
                        current.Services[i].Status = newStatus
                        current.Services[i].NewStatus = true
                    }
                }

            }
            if err != nil {
                current.Info = err.Error()
            } else {
                current.Info = ""
            }
        }
    }
}

func (p *MemoryMonitoredInstances) GetAppStatus(appInstanceId string) (*entities.FragmentStatus, error){
    p.mu.Lock()
    defer p.mu.Unlock()
    for _, current := range p.monitoredEntries {
        // all deployment fragments belong to the same appInstance, the first found value must be the same for all
        if current.AppInstanceId == appInstanceId {
            return &current.Status, nil
        }
    }

    return nil, errors.New(fmt.Sprintf("cannot get status of app %s because it does not exist", appInstanceId))

}


// Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
// If after the maximum expiration time the check is not successful, the execution is considered to be failed.
func(p *MemoryMonitoredInstances) WaitPendingChecks(fragmentId string, checkingSleepTime int, stageCheckingTimeout int) error {
    log.Info().Msgf("fragment %s wait until services for the instance are ready", fragmentId)
    timeout := time.After(time.Second * time.Duration(stageCheckingTimeout))
    tick := time.Tick(time.Second * time.Duration(checkingSleepTime))
    for {
        select {
        // Got a timeout! Error
        case <-timeout:
            log.Error().Str("fragmentId", fragmentId).Msg("checking pendingStages resources exceeded for stage")
            return errors.New(fmt.Sprintf("checking pendingStages resources exceeded for fragment %s", fragmentId))
            // Next check
        case <-tick:
            p.UpdateAppStatus(fragmentId)
            monitoredEntry, found := p.monitoredEntries[fragmentId]
            if !found {
                log.Info().Str("fragmentId", fragmentId).Msg("fragment not monitored")
                return errors.New(fmt.Sprintf("not monitored fragment %s", fragmentId))
            }
            if monitoredEntry.NumPendingChecks == 0 {
                log.Info().Str("fragmentId",fragmentId).Msg("fragment has no pendingStages checks. Exit checking stage")
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

    appEntry, found := p.monitoredEntries[newResource.FragmentId]
    if !found {
        log.Error().Str("appInstanceID", newResource.FragmentId).Msg("impossible to add resource. Fragment not monitored.")
        return false
    }

    // Get the service
    service, found := appEntry.Services[newResource.ServiceInstanceID]
    if !found {
        log.Error().Str("fragmentId", newResource.FragmentId).Str("serviceInstanceID", newResource.ServiceInstanceID).
            Msg("impossible to add resource. Service not monitored.")
        return false
    }
    service.AddPendingResource(newResource)

    log.Debug().Str("fragmentId", newResource.FragmentId).Int("pending checks",service.NumPendingChecks).
        Msg("a new resource has been added")
    return true
}

func(p *MemoryMonitoredInstances) RemovePendingResource(fragmentId string, serviceInstanceID string, uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    appEntry, found := p.monitoredEntries[fragmentId]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Msg("impossible to remove resource. Fragment not monitored.")
        return false
    }

    // Get the service
    service, found := appEntry.Services[serviceInstanceID]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Str("serviceInstanceID", serviceInstanceID).
            Msg("impossible to remove resource. Service not monitored.")
        return false
    }

    // -> resource
    pendingResource, found := service.Resources[uid]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Str("serviceInstanceID", serviceInstanceID).
            Str("resource uid", uid).Msg("impossible to remove resource. Resource not monitored")
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
func (p *MemoryMonitoredInstances) IsMonitoredResource(fragmentId string, serviceInstanceID string, uid string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    appEntry, found := p.monitoredEntries[fragmentId]
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



func (p *MemoryMonitoredInstances) SetResourceStatus(fragmentId string, serviceInstanceId, uid string,
    status entities.NalejServiceStatus, info string, endpoints []entities.EndpointInstance) {
    p.mu.Lock()
    defer p.mu.Unlock()

    log.Debug().Str("fragmentId", fragmentId).Str("serviceInstanceID", serviceInstanceId).Str("uid",uid).
        Interface("status",status).Str("info",info).Msg("set resource status")

    app, found := p.monitoredEntries[fragmentId]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Msg("impossible to set resource status. App not monitored.")
        return
    }

    // Get the service
    service, found := app.Services[serviceInstanceId]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Str("serviceInstanceID", serviceInstanceId).
            Interface("monitored",p.monitoredEntries).Msg("impossible to set resource. Service not monitored.")
        return
    }

    // -> resource
    resource, found := service.Resources[uid]
    if !found {
        log.Warn().Str("fragmentId", fragmentId).Str("serviceInstanceId", serviceInstanceId).Str("resource uid", uid).
            Msg("resource was not added before setting a new status. We add it now")

        newResource := entities.NewMonitoredPlatformResource(fragmentId, uid, service.AppDescriptorId, service.AppInstanceId,
            service.ServiceGroupId, service.ServiceGroupInstanceId, service.ServiceID, service.ServiceInstanceID, info)
        service.AddPendingResource(&newResource)
        resource = service.Resources[uid]
    }

    // If we are going to set the same status, exit.
    if resource.Status == status {
        log.Debug().Str("fragmentId", fragmentId).Str("serviceInstanceId", serviceInstanceId).Str("uid",uid).
            Interface("status",status).Str("info",info).Msg("no resource status changed")
        return
    }

    // Modify the status
    // If this resource goes into a non-running state, we have a new pending check
    if resource.Status == entities.NALEJ_SERVICE_RUNNING {
        service.NumPendingChecks++
    }
    resource.Status = status
    resource.Info = info

    // set the endpoints for this entry
    if len(endpoints) >0 {
        if service.Endpoints == nil {
            service.Endpoints = endpoints
        } else {
            // add the endpoints
            service.Endpoints = append(service.Endpoints, endpoints...)
        }
    }


    // If this is running remove one check
    if resource.Status == entities.NALEJ_SERVICE_RUNNING {
        log.Debug().Str("fragmentId", fragmentId).Str("serviceInstanceId", serviceInstanceId).Str("uid",uid).
            Interface("status",status).Str("info",info).Msg("resource is running, stop monitoring it")
        service.RemovePendingResource(resource.UID)
    }

    // Update service status
    // get the worst status found in the resources required by this service
    previousStatus := service.Status
    var finalStatus entities.NalejServiceStatus
    finalStatus = entities.NALEJ_SERVICE_RUNNING
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
        log.Debug().Str("serviceInstanceID", service.ServiceInstanceID).Interface("status",finalStatus).
            Msg("service changed status")
        service.NewStatus = true
        if previousStatus == entities.NALEJ_SERVICE_RUNNING {
            // we have change the status but this service was already running monitor it again
            log.Debug().Str("serviceInstanceID", service.ServiceInstanceID).Interface("status",finalStatus).
                Msg("service stopped working, monitor it")
        }
    }

    service.Status = finalStatus
    service.Info = newServiceInfo

    var newAppStatus entities.NalejServiceStatus
    newAppStatus = entities.NALEJ_SERVICE_RUNNING
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

    log.Debug().Interface("previous",app.Status).Interface("now",newAppStatus).
        Msg("final status of the app after updating services")

    app.Status = entities.ServicesToFragmentStatus[newAppStatus]
    app.Info = newAppInfo
}

func (p *MemoryMonitoredInstances) GetPendingNotifications() ([] *entities.MonitoredAppEntry) {
    p.mu.RLock()
    toReturn := make([]*entities.MonitoredAppEntry,0)
    // list of apps to be removed
    toRemove := make([]string, 0)
    for _, entry := range p.monitoredEntries {
        if entry.Status == entities.FRAGMENT_TERMINATING {
            toRemove = append(toRemove, entry.FragmentId)
        }
        pendingServices := make(map[string]*entities.MonitoredServiceEntry,0)
        // for every monitored entry
        for _, x := range entry.Services {
            // for every monitored service
            if entry.NewStatus || x.NewStatus {
                pendingServices[x.ServiceInstanceID] = x
            }
        }
        if len(pendingServices) > 0 {
            newApp := entities.MonitoredAppEntry{
                FragmentId:       entry.FragmentId,
                NumPendingChecks: entry.NumPendingChecks,
                OrganizationId:   entry.OrganizationId,
                AppInstanceId:    entry.AppInstanceId,
                Services:         pendingServices,
                Info:             entry.Info,
                DeploymentId:     entry.DeploymentId,
                Status:           entry.Status,
                AppDescriptorId:  entry.AppDescriptorId,
            }
            toReturn = append(toReturn, &newApp)
        }
    }
    // remove entries in terminating status
    for _, fragmentId := range toRemove {
        log.Debug().Str("fragmentId", fragmentId).Msg("remove terminating entry from the list of monitored")
        defer p.RemoveEntry(fragmentId)
    }
    // This is the latest unlock to be deferred to respect the order and avoid race conditions
    defer p.mu.RUnlock()
    return toReturn
}

// Return the list of services with a new status to be notified.
// returns:
//  array with the collection of entities with a service with a status pending of notification
func (p *MemoryMonitoredInstances) GetServicesUnnotifiedStatus() [] *entities.MonitoredServiceEntry {
    p.mu.RLock()
    defer p.mu.RUnlock()
    toNotify := make([]*entities.MonitoredServiceEntry,0)
    for _, app := range p.monitoredEntries {
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
    defer p.mu.Unlock()
    for _, stage := range p.monitoredEntries {
        stage.NewStatus = false
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

func (p *MemoryMonitoredInstances) UpdateAppStatus(fragmentId string) {
    app, found := p.monitoredEntries[fragmentId]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Msg("impossible to update app status. App not monitored.")
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
        log.Info().Str("fragmentId", fragmentId).Int("pendingServices",app.NumPendingChecks).
            Msg("updated number of pending services for app")
    }
}


func (p *MemoryMonitoredInstances) RemoveEntry(fragmentId string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()
    log.Debug().Str("fragmentId", fragmentId).Msg("remove app from the list of monitored")

    _, found := p.monitoredEntries[fragmentId]
    if !found {
        log.Error().Str("fragmentId", fragmentId).Msg("impossible to delete monitored entry. App not found")
        return false
    }

    delete(p.monitoredEntries, fragmentId)
    return true
}


func (p *MemoryMonitoredInstances) GetNumFragments() int {
    return len(p.monitoredEntries)
}

func (p *MemoryMonitoredInstances) GetNumApps() int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    list := make(map[string]bool,0)
    for _, entry := range p.monitoredEntries {
        if _, found := list[entry.FragmentId]; !found {
            list[entry.FragmentId] = true
        }
    }
    return len(list)
}


func (p *MemoryMonitoredInstances) GetNumServices() int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    accum := 0
    for _, entry := range p.monitoredEntries {
        accum = accum + len(entry.Services)
    }
    return accum
}

func (p *MemoryMonitoredInstances) GetNumResources() int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    accum := 0
    for _, entry := range p.monitoredEntries {
        for _, serv := range entry.Services {
            accum = accum + len(serv.Resources)
        }
    }
    return accum
}