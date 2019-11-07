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
