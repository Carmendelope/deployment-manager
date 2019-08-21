/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Metrics provider interface

package metrics

import (
	"net/http"
)

// A MetricsProvider is able to return the data for an endpoint that a scraper
// can use. Depending on the implementation, it can get this data either by
// interfacing with GetMetrics on an EventsProvider, by sharing a Collector
// with an EventsProvider, or through some other mechanism (e.g., a global).
type MetricsProvider interface {
	// Return the scraper data
	Metrics(w http.ResponseWriter, r *http.Request)
}
