/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
	collector       metrics.Collector
}

func NewManager(metricsProvider metrics.MetricsProvider, collector metrics.Collector) (*Manager, derrors.Error) {
	manager := &Manager{
		metricsProvider: metricsProvider,
		collector:       collector,
	}

	return manager, nil
}

func (m *Manager) Start() derrors.Error {
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
		for t, m := range metrics {
			log.Debug().
				Int64("created", m.Created).
				Int64("deleted", m.Deleted).
				Int64("running", m.Running).
				Int64("errors", m.Errors).
				Msg(string(t))
		}
	}
}
