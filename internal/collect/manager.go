/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 */

// Collect manager responsible for collecting platform-relevant monitoring
// data and storing it in-memory, ready to be exported to a data store

package collect

import (
	"net/http"
	"time"

	"github.com/nalej/derrors"

	"github.com/nalej/deployment-manager/pkg/metrics"

	"github.com/rs/zerolog/log"
)

type Manager struct {
	metricsProvider metrics.MetricsProvider
	collector metrics.Collector
}

func NewManager(metricsProvider metrics.MetricsProvider, collector metrics.Collector) (*Manager, derrors.Error) {
	manager := &Manager{
		metricsProvider: metricsProvider,
		collector: collector,
	}

	return manager, nil
}

func (m *Manager) Start() (derrors.Error) {
	log.Debug().Msg("starting metrics manager")

	if log.Debug().Enabled() {
		go m.tickerLogger(time.Tick(time.Minute))
	}

	return nil
}

func (m *Manager) Metrics(w http.ResponseWriter, r *http.Request) {
	m.metricsProvider.Metrics(w, r)
}

func (m *Manager) tickerLogger(c <-chan time.Time) {
	for {
		<-c
		metrics, _ := m.collector.GetMetrics()
		for t, m := range(metrics) {
			log.Debug().
				Int64("created", m.Created).
				Int64("deleted", m.Deleted).
				Int64("running", m.Running).
				Int64("errors", m.Errors).
				Msg(string(t))
		}
	}
}
