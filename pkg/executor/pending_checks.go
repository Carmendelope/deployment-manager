/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package executor

import (
    "errors"
    "sync"
    "time"
    "fmt"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/common"
)


const (
    // Time in seconds we wait for a stage to be finished.
    StageCheckingTimeout = 120
    // Time between pending stage checks in seconds
    CheckingSleepTime = 1
)

// We store the number of pending checks for a certain stage. Every time a check is done, we reduce the number
// of pending stages until, it's zero. For every pending resource we store an inverted pointer to the stage it
// belongs to.
type PendingStages struct {
    // Organization Id these stages are running into
    OrganizationId string
    // Instance Id these stages belong to
    InstanceId string
    // fragment id for these stages
    FragmentId string
    // Main deployable object
    ToDeploy Deployable
    // nalej stage -> num pending checks
    stagePendingChecks map[string]int
    // platform resource uid -> parent stage
    resourceStage map[string]string
    // native resourceId -> serviceId
    resourceService map[string]string
    // serviceId -> [resourceId_0, resourceId_1, etc.]
    serviceResources map[string][]string
    // native resourceId -> status
    resourceStatus map[string]entities.NalejServiceStatus
    // serviceId -> status
    serviceStatus map[string]entities.NalejServiceStatus
    mu               sync.RWMutex
    // conductor monitor client
    monitor Monitor
}


// Create a new set of pending stages with a monitor helper to inform conductor about the current status.
func NewPendingStages(organizationId string, instanceId string, fragmentId string, toDeploy Deployable,
    monitor Monitor) *PendingStages {
    return &PendingStages{
        OrganizationId:     organizationId,
        InstanceId:         instanceId,
        FragmentId:         fragmentId,
        ToDeploy:           toDeploy,
        stagePendingChecks: make(map[string]int,0),
        resourceStage:      make(map[string]string,0),
        resourceService:    make(map[string]string,0),
        serviceResources:   make(map[string][]string,0),
        resourceStatus:     make(map[string]entities.NalejServiceStatus),
        serviceStatus:      make(map[string]entities.NalejServiceStatus),
        monitor:            monitor,
    }
}


// Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
// If after the maximum expiration time the check is not successful, the execution is considered to be failed.
func(p *PendingStages) WaitPendingChecks(stageId string) error {
    log.Info().Msgf("stage %s wait until all stages are complete",stageId)
    timeout := time.After(time.Second * StageCheckingTimeout)
    tick := time.Tick(time.Second * CheckingSleepTime)
    for {
        select {
        // Got a timeout! Error
        case <-timeout:
            log.Error().Msgf("checking pendingStages resources exceeded for stage %s", stageId)
            return errors.New(fmt.Sprintf("checking pendingStages resources exceeded for stage %s", stageId))
            // Next check
        case <-tick:
            pendingStages := p.StageHasPendingChecks(stageId)
            if !pendingStages {
                log.Info().Msgf("stage %s has no pendingStages checks. Exit checking stage", stageId)
                return nil
            }
        }
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
        items := p.serviceResources[serviceId]
        items = append(items, uid)
        p.serviceResources[serviceId] = items
    }
    log.Debug().Msgf("resources for service %s are %v",serviceId, p.serviceResources[serviceId])
    p.serviceStatus[serviceId] = entities.NALEJ_SERVICE_SCHEDULED

}


// Remove a resource from the pending list. Return false if the resource is not there.
func(p *PendingStages) RemoveResource(uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    // remove from the observed services
    delete(p.resourceStatus,uid)
    serviceId := p.resourceService[uid]
    delete(p.resourceService,uid)
    newResources := p.serviceResources[serviceId]
    for index, res := range newResources {
        if res == uid {
            newResources = append(newResources[:index], newResources[index+1:]...)
            break
        }
    }
    if len(newResources) == 0 {
        // no more pending newResources for this service
        log.Debug().Msgf("no more pending newResources for service %s", serviceId)
        delete(p.serviceResources,serviceId)
    } else {
        p.serviceResources[serviceId] = newResources
    }


    stage,isthere := p.resourceStage[uid]
    if !isthere {
        log.Error().Msgf("impossible to remove resource %s. It had no associated stage", uid)
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
//  endpoints optional array of endpoints
func (p *PendingStages) SetResourceStatus(uid string, status entities.NalejServiceStatus) {
    p.mu.Lock()
    defer p.mu.Unlock()
    // get associated service id
    serviceId, isthere := p.resourceService[uid]
    if !isthere{
        log.Error().Msgf("trying to set resource status for resource %s without service", uid)
        return
    }
    // update status
    p.resourceStatus[uid] = status
    // get the worst status found in the resources required by this service
    var finalStatus entities.NalejServiceStatus
    finalStatus = entities.NALEJ_SERVICE_ERROR
    //log.Debug().Msgf("check service %s with resources %v", serviceId, p.resourceService[serviceId])
    for _,resourceId := range p.serviceResources[serviceId] {
        //log.Debug().Msgf("--> resource %s has status %v",resourceId, p.resourceStatus[resourceId])
        if p.resourceStatus[resourceId] == entities.NALEJ_SERVICE_ERROR {
            finalStatus = entities.NALEJ_SERVICE_ERROR
            break
        } else if p.resourceStatus[resourceId] < finalStatus {
            finalStatus = p.resourceStatus[resourceId]
        }

    }
    //log.Debug().Msgf("finally service %s has status %v", serviceId, finalStatus)
    p.serviceStatus[serviceId] = finalStatus
    // Do not communicate updates regarding all services.
    if serviceId != common.AllServices {
        p.monitor.UpdateServiceStatus(p.FragmentId,p.OrganizationId, p.InstanceId,
            serviceId,p.serviceStatus[serviceId],p.ToDeploy)
    }
}