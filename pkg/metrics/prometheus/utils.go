/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Prometheus utils

package prometheus

import (
	"fmt"
	"math"
	"strings"

	"github.com/nalej/derrors"
	"github.com/nalej/deployment-manager/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
)

// This function turned out way too long - it's only used for some debug logging :(
func parseMetrics(pMetrics []*dto.MetricFamily) (metrics.Metrics, derrors.Error) {
	parsedMetrics := metrics.Metrics{}
	for _, pMetric := range(pMetrics) {
		// Parse the metric name into metric types and the specific counter
		metricSplits := strings.Split(*pMetric.Name, "_")
		if len(metricSplits) < 2 {
			return nil, derrors.NewInternalError("invalid metrics returned from registry")
		}
		metricType := metrics.MetricType(metricSplits[0])
		metricCounter := metrics.MetricCounter(metricSplits[1])

		// Create the output metric if needed
		metric, found := parsedMetrics[metricType]
		if !found {
			metric = &metrics.Metric{}
			parsedMetrics[metricType] = metric
		}

		var value int64
		var err error
		switch *pMetric.Type {
		case dto.MetricType_COUNTER:
			value, err = Ftoi(*pMetric.Metric[0].Counter.Value)
		case dto.MetricType_GAUGE:
			value, err = Ftoi(*pMetric.Metric[0].Gauge.Value)
		default:
			return nil, derrors.NewInternalError(fmt.Sprintf("unsupported prometheus metric type %d", pMetric.Type))
		}
		if err != nil {
			return nil, derrors.NewInternalError("error converting value", err)
		}

		switch metricCounter {
		case metrics.MetricCreated:
			metric.Created = value
		case metrics.MetricDeleted:
			metric.Deleted = value
		case metrics.MetricErrors:
			metric.Errors = value
		case metrics.MetricRunning:
			metric.Running = value
		}
	}

	return parsedMetrics, nil
}

func createSubsystem(t metrics.MetricType) *Subsystem {
	log.Debug().Str("metric", string(t)).Msg("creating collectors")
	opts := prometheus.Opts{
		Subsystem: string(t),
	}

	createdOpts := prometheus.CounterOpts(opts)
	createdOpts.Name = fmt.Sprintf("%s_total", metrics.MetricCreated)

	deletedOpts := prometheus.CounterOpts(opts)
	deletedOpts.Name = fmt.Sprintf("%s_total", metrics.MetricDeleted)

	errorsOpts := prometheus.CounterOpts(opts)
	errorsOpts.Name = fmt.Sprintf("%s_total", metrics.MetricErrors)

	runningOpts := prometheus.GaugeOpts(opts)
	runningOpts.Name = string(metrics.MetricRunning)

	return &Subsystem{
		Created: prometheus.NewCounter(createdOpts),
		Deleted: prometheus.NewCounter(deletedOpts),
		Errors: prometheus.NewCounter(errorsOpts),
		Running: prometheus.NewGauge(runningOpts),
	}
}

func Ftoi(f float64) (int64, error) {
	if math.IsNaN(f) {
		return 0, nil
	}

	i := int64(f)
	if float64(i) != math.Trunc(f) {
		return 0, fmt.Errorf("float %f out of int64 range", f)
	}
	return i, nil
}
