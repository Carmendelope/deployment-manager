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
    mu sync.RWMutex
    // conductor monitor client
    monitor *monitor.MonitorHelper
}


// Create a new set of pending stages with a monitor helper to inform conductor about the current status.
func NewPendingStages(fragmentId string, monitor *monitor.MonitorHelper) *PendingStages {
    return &PendingStages{
        FragmentId: fragmentId,
        stagePendingChecks: make(map[string]int,0),
        resourceStage: make(map[string]string,0),
        monitor: monitor,
    }
}

// Add a new resource pending to be checked.
func(p *PendingStages) AddResource(uid string, stageId string) {
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
}


// Remove a resource from the pending list. Return false if the resource is not there.
func(p *PendingStages) RemoveResource(uid string) bool {
    p.mu.Lock()
    defer p.mu.Unlock()
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

func(p *PendingStages) HasPendingChecks(stage string) bool {
    p.mu.RLock()
    defer p.mu.RUnlock()
    _, isthere := p.stagePendingChecks[stage]
    return isthere
}


