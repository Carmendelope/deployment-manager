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
