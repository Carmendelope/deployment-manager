/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    "sync"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/pkg/monitor"
    "github.com/nalej/deployment-manager/internal/entities"
)

// We store the number of pending checks for a certain stage. Every time a check is done, we reduce the number
// of pending stages until, it's zero. For every pending resource we store an inverted pointer to the stage it
// belongs to.
type PendingStages struct {
    // fragment id for these stages
    FragmentId string
    // nalej stage -> num pending checks
    stagePendingChecks map[string]int
    // platform resource uid -> parent stage
    resourceStage map[string]string
    // native resourceId -> serviceId
    resourceService map[string]string
    // serviceId -> [resourceId_0, resourceId_1, etc.]
    serviceResources map[string][]string
    // native resourceId -> status
    resourceStatus map[string]entities.ServiceStatus
    // serviceId -> status
    serviceStatus map[string]entities.ServiceStatus
    mu               sync.RWMutex
    // conductor monitor client
    monitor *monitor.MonitorHelper
}


// Create a new set of pending stages with a monitor helper to inform conductor about the current status.
func NewPendingStages(fragmentId string, monitor *monitor.MonitorHelper) *PendingStages {
    return &PendingStages{
        FragmentId:         fragmentId,
        stagePendingChecks: make(map[string]int,0),
        resourceStage:      make(map[string]string,0),
        resourceService:    make(map[string]string,0),
        serviceResources:   make(map[string][]string,0),
        resourceStatus:     make(map[string]entities.ServiceStatus),
        monitor:            monitor,
    }
}

// Add a new resource pending to be checked.
func(p *PendingStages) AddMonitoredResource(uid string, serviceId string, stageId string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.resourceStage[uid] = stageId
    currentChecks, isThere := p.stagePendingChecks[stageId]
    if !isThere {
        currentChecks =1
    } else {
        currentChecks = currentChecks + 1
    }
    log.Debug().Msgf("stage %s has %d pending checks after adding %s", stageId, currentChecks, uid)
    p.stagePendingChecks[stageId] = currentChecks
    // ---
    p.resourceService[uid] = serviceId
    _, isThere = p.serviceResources[serviceId]
    if !isThere {
        p.serviceResources[serviceId] = []string{uid}
    } else {
        p.serviceResources[serviceId] = append(p.serviceResources[serviceId], uid)
    }
}


// Remove a resource from the pending list. Return false if the resource is not there.
func(p *PendingStages) RemoveResource(uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    // remove from the observed services
    serviceId := p.resourceService[uid]
    delete(p.resourceService,uid)
    resources := p.serviceResources[serviceId]
    for index, res := range resources {
        if res == uid {
            resources = append(resources[:index],resources[index+1:]...)
            break
        }
    }
    if len(resources) == 0 {
        // no more pending resources for this service
        log.Debug().Msgf("no more pending resources for service %s", serviceId)
        delete(p.serviceResources,serviceId)
    }

    stage,isthere := p.resourceStage[uid]
    if !isthere {
        return false
    }
    // delete the entry
    delete(p.resourceStage,uid)

    // remove one from the stage pending checks
    numChecks, isthere := p.stagePendingChecks[stage]
    if isthere {
        numChecks = numChecks - 1
    } else {
        log.Error().Msgf("stage %s has no registered pending checks", stage)
        return false
    }

    if numChecks == 0 {
        delete(p.stagePendingChecks,stage)
        log.Debug().Msgf("stage %s has no more pending. We delete it", stage)
    } else {
        p.stagePendingChecks[stage] = numChecks
        log.Debug().Msgf("stage %s has %d pending checks", stage, numChecks)
    }
    return true
}

// Return true if the passed uid corresponds to a resource being monitored.
func (p *PendingStages) IsMonitoredResource(uid string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    _, isthere := p.resourceStage[uid]
    return isthere
}

func(p *PendingStages) StageHasPendingChecks(stage string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    _, isthere := p.stagePendingChecks[stage]
    return isthere
}

func (p *PendingStages) ServiceHasPendingChecks(serviceId string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    _, isthere := p.serviceResources[serviceId]
    return isthere
}


// Set the status of a resource. This function determines how to change the service status
// depending on the combination of the statuses of its related resources.
// params:
//  uid native resource identifier
//  status of the native resource
func (p *PendingStages) SetResourceStatus(uid string, status entities.ServiceStatus) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    // get associated service id
    serviceId, isthere := p.resourceService[uid]
    if !isthere{
        log.Error().Msgf("trying to set resource status for resource without service")
        return
    }
    // update status
    p.resourceStatus[uid] = status
    // get the worst status found in the resources required by this service
    var finalStatus entities.ServiceStatus
    finalStatus = entities.SERVICE_ERROR
    for _,resourceId := range p.serviceResources[serviceId] {
        if p.resourceStatus[resourceId] == entities.SERVICE_ERROR {
            finalStatus = entities.SERVICE_ERROR
            break
        } else if p.resourceStatus[resourceId] < finalStatus {
            finalStatus = p.resourceStatus[resourceId]
        }
    }
    p.serviceStatus[serviceId] = finalStatus
    p.monitor.UpdateServiceStatus(p.FragmentId,serviceId,finalStatus)
}