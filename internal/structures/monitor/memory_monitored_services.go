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
    // nalej stage -> monitored services
    monitoredStages map[string]entities.MonitoredStageEntry
    // Set of platform resources monitored
    // resource uid -> resource data
    monitoredPlatformResources map[string]entities.MonitoredPlatformResource
    // Mutex
    mu sync.RWMutex
}

// Constructor to instantiate a basic memory monitored instances object.
func NewMemoryMonitoredInstances() MonitoredInstances {
    return &MemoryMonitoredInstances{
        monitoredStages: make(map[string]entities.MonitoredStageEntry,0),
        monitoredPlatformResources: make(map[string]entities.MonitoredPlatformResource,0),
    }
}

func (p *MemoryMonitoredInstances) AddStage(toAdd entities.MonitoredStageEntry) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.monitoredStages[toAdd.StageID] = toAdd
}

// Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
// If after the maximum expiration time the check is not successful, the execution is considered to be failed.
func(p *MemoryMonitoredInstances) WaitPendingChecks(stageId string, checkingSleepTime int, stageCheckingTimeout int) error {
    log.Info().Msgf("stage %s wait until all stages are complete",stageId)
    timeout := time.After(time.Second * time.Duration(stageCheckingTimeout))
    tick := time.Tick(time.Second * time.Duration(checkingSleepTime))
    for {
        select {
        // Got a timeout! Error
        case <-timeout:
            log.Error().Str("stageID",stageId).Msg("checking pendingStages resources exceeded for stage")
            return errors.New(fmt.Sprintf("checking pendingStages resources exceeded for stage %s", stageId))
            // Next check
        case <-tick:
            pendingStages := p.StageHasPendingChecks(stageId)
            if !pendingStages {
                log.Info().Str("stageID",stageId).Msg("stage %s has no pendingStages checks. Exit checking stage")
                return errors.New(fmt.Sprintf("stage %s has no pendingStages checks. Exit checking stage", stageId))
            }
        }
    }
}



// Add a new resource pending to be checked.
func(p *MemoryMonitoredInstances) AddPendingResource(newResource entities.MonitoredPlatformResource) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Get all the monitored entries for the stage
    stageData, found := p.monitoredStages[newResource.StageID]
    if !found {
        log.Error().Str("stageID", newResource.StageID).Msg("stage not monitored")
        return
    }
    stageData.AddPendingResource(newResource)

    log.Debug().Str("stageId",newResource.StageID).Int("pending checks",stageData.NumPendingChecks).
        Msg("resources have been extended")
}

func(p *MemoryMonitoredInstances) RemovePendingResource(uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()
    pendingResource, found := p.monitoredPlatformResources[uid]
    if !found {
        log.Error().Str("resource uid", uid).Msg("pending resource not found")
        return false
    }
    // This is not pending
    pendingResource.Pending = false
    // One less check to fulfill
    stageID := pendingResource.StageID
    // remove one entry from the pending list
    entry, found := p.monitoredStages[stageID]
    if !found {
        log.Info().Str("stageID",stageID).Msg("stage %s has no pendingStages checks. Exit checking stage")
        return false
    }
    entry.NumPendingChecks = entry.NumPendingChecks - 1
    return true
}


// Return true if the passed uid corresponds to a resource being monitored.
func (p *MemoryMonitoredInstances) IsMonitoredResource(uid string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    _, isthere := p.monitoredPlatformResources[uid]

    return isthere
}

func(p *MemoryMonitoredInstances) StageHasPendingChecks(stageID string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    monitored, isthere := p.monitoredStages[stageID]
    if !isthere {
        log.Error().Str("stageID",stageID).Msg("stage is not monitored")
        return false
    }
    return monitored.NumPendingChecks > 0
}


// Set the status of a resource. This function determines how to change the service status
// depending on the combination of the statuses of its related resources.
// params:
//  uid native resource identifier
//  status of the native resource
//  endpoints optional array of endpoints
func (p *MemoryMonitoredInstances) SetResourceStatus(uid string, status entities.NalejServiceStatus, info string) {
    p.mu.Lock()
    defer p.mu.Unlock()

    resource, found := p.monitoredPlatformResources[uid]
    if !found {
        log.Error().Str("uid", uid).Msg("not registered resource")
        return
    }

    // Modify the status
    resource.Status = status
    resource.Info = info
    // Get the stage
    stage, found := p.monitoredStages[resource.StageID]
    if !found {
        log.Error().Str("stageID",resource.StageID).Msg("stage is not monitored")
        return
    }
    // Get the service
    service, found := stage.Services[resource.ServiceID]
    if !found {
        log.Error().Str("stageID",resource.StageID).Str("serviceID",resource.ServiceID).Msg("service is not monitored")
        return
    }

    // If this is running remove one check
    if resource.Status == entities.NALEJ_SERVICE_RUNNING {
        service.NumPendingChecks = service.NumPendingChecks - 1
    }

    // Update service status
    // get the worst status found in the resources required by this service
    previousStatus := service.Status
    var finalStatus entities.NalejServiceStatus
    finalStatus = entities.NALEJ_SERVICE_ERROR
    newServiceInfo := service.Info
    //log.Debug().Msgf("check service %s with resources %v", serviceId, p.resourceService[serviceId])
    for _,res := range service.Resources {
        //log.Debug().Msgf("--> resource %s has status %v",resourceId, p.resourceStatus[resourceId])
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
        service.NewStatus = true
    }
    log.Debug().Str("serviceID", service.ServiceID).Str("status",string(finalStatus)).Msg("service changed status")
    service.Status = finalStatus
    service.Info = newServiceInfo
}

// Return the list of services with a new status to be notified.
// returns:
//  array with the collection of entities with a service with a status pending of notification
func (p *MemoryMonitoredInstances) GetServicesUnnotifiedStatus() [] entities.MonitoredServiceEntry {
    toNotify := make([]entities.MonitoredServiceEntry,0)
    for _, stage := range p.monitoredStages {
        // for every monitored stage
        for _, x := range stage.Services {
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
    for _, stage := range p.monitoredStages {
        // for every monitored stage
        for _, x := range stage.Services {
            // for every monitored service
            if x.NewStatus {
                x.NewStatus = false
            }
        }
    }
}