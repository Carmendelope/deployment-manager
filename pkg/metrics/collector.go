/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Interface for a collector that stores metrics

package metrics

import (
	"github.com/nalej/derrors"
)

// Interface to collect creation/deletion events of metrics.
// NOTE: We assume these are not thread-safe and should not be called
// concurrently
type Collector interface {
	// A resource for a metric has been created
	Create(t MetricType)
	// A resource has been created before monitoring started, so should be
	// counted as running but not created
	Existing(t MetricType)
	// A resource for a metric has been deleted
	Delete(t MetricType)
	// A resource for a metric has encountered an error
	Error(t MetricType)
	// Get all current metrics
	GetMetrics(types ...MetricType) (Metrics, derrors.Error)
}
