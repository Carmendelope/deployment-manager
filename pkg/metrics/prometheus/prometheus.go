/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Prometheus implementation for metrics interface

package prometheus

import (
	"net/http"

	"github.com/nalej/derrors"
	"github.com/nalej/deployment-manager/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type MetricsProvider struct {
	// Local prometheus registry
	registry *prometheus.Registry
	// Map of collectors - each metric has a few collectors and we will
	// create and register these on-the-fly.
	subsystems map[metrics.MetricType]*Subsystem
	// Handler provided by prometheus
	handler http.Handler
}

type Subsystem struct {
	Created, Deleted, Errors prometheus.Counter
	Running prometheus.Gauge
}

func NewMetricsProvider() (*MetricsProvider, derrors.Error) {
	log.Debug().Msg("creating prometheus metrics provider")

	registry := prometheus.NewRegistry()

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
	})

	provider := &MetricsProvider{
		registry: registry,
		subsystems: map[metrics.MetricType]*Subsystem{},
		handler: handler,
	}

	return provider, nil
}

func (p *MetricsProvider) Metrics(w http.ResponseWriter, r *http.Request) {
	p.handler.ServeHTTP(w, r)
}

func (p *MetricsProvider) Create(t metrics.MetricType) {
	log.Debug().Str("metric", string(t)).Msg("created")
	subsystem := p.getSubsystem(t)

	subsystem.Created.Inc()
	subsystem.Running.Inc()
}

func (p *MetricsProvider) Existing(t metrics.MetricType) {
	log.Debug().Str("metric", string(t)).Msg("existing")
	subsystem := p.getSubsystem(t)

	subsystem.Running.Inc()
}

func (p *MetricsProvider) Delete(t metrics.MetricType) {
	log.Debug().Str("metric", string(t)).Msg("deleted")
	subsystem := p.getSubsystem(t)

	subsystem.Deleted.Inc()
	subsystem.Running.Dec()
}

func (p *MetricsProvider) Error(t metrics.MetricType) {
	log.Debug().Str("metric", string(t)).Msg("error")
	subsystem := p.getSubsystem(t)

	subsystem.Errors.Inc()
}

func (p *MetricsProvider) GetMetrics(types ...metrics.MetricType) (metrics.Metrics, derrors.Error) {
	pMetrics, err := p.registry.Gather()
	if err != nil {
		return nil, derrors.NewInternalError("failed gathering prometheus metrics", err)
	}

	return parseMetrics(pMetrics)
}

// As we implement Collector, we can just return ourselves
func (p *MetricsProvider) GetCollector() metrics.Collector {
	return p
}

func (p *MetricsProvider) getSubsystem(t metrics.MetricType) (*Subsystem) {
	subsystem, found := p.subsystems[t]
	if !found {
		// NOTE/WARNING: We can do this without locking the map,
		// because we know create/delete is called in a serial
		// fashion. Our Kubernetes Event provider uses a workqueue
		// and single thread to fire off these calls.
		subsystem = createSubsystem(t)
		p.subsystems[t] = subsystem

		// If we fail doing this, why bother continuing - panic
		p.registry.MustRegister(subsystem.Created, subsystem.Deleted, subsystem.Errors, subsystem.Running)
	}

	return subsystem
}
