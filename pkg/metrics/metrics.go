/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Structs for platform metrics for collecting and querying

package metrics

// List of available metrics
type MetricType string
const (
	MetricServices MetricType = "services"
	MetricVolumes MetricType = "volumes"
	MetricFragments MetricType = "fragments"
	MetricEndpoints MetricType = "endpoints"
)

func (m MetricType) String() string {
	return string(m)
}

// String references for the counters in a metric
type MetricCounter string
const (
	MetricCreated MetricCounter = "created"
	MetricDeleted MetricCounter = "deleted"
	MetricErrors MetricCounter = "errors"
	MetricRunning MetricCounter = "running"
)

func (m MetricCounter) String() string {
	return string(m)
}

// Individual metric
// NOTE: At this point we log warnings as errors. For true errors we would
// need to decide what an error actually is (unavailable container or endpoint?
// application that quits unexpectedly?), if it's transient or permanent,
// whether we actually care about it, etc. Then we'd need to analyze the event
// and other resources to figure out what we're dealing with. So, for now, we
// just count warnings.
type Metric struct {
	Created, Deleted, Running, Errors int64
}
// Metrics collection
type Metrics map[MetricType]*Metric
