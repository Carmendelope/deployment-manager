/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Prometheus metrics collector test

package prometheus

import (
	"github.com/nalej/derrors"

	"github.com/nalej/deployment-manager/pkg/metrics"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("prometheus", func() {

	var provider *MetricsProvider
	var collector metrics.Collector

	ginkgo.BeforeSuite(func() {
		var derr derrors.Error
		provider, derr = NewMetricsProvider()
		gomega.Expect(derr).To(gomega.Succeed())

		collector = provider.GetCollector()
	})

	ginkgo.It("should count metrics correctly", func() {
		collector.Existing(metrics.MetricVolumes)
		gomega.Expect(provider.GetMetrics()).To(gomega.Equal(metrics.Metrics{
			metrics.MetricVolumes: &metrics.Metric{
				Created: 0,
				Deleted: 0,
				Errors: 0,
				Running: 1,
			},
		}))

		collector.Create(metrics.MetricVolumes)
		gomega.Expect(provider.GetMetrics()).To(gomega.Equal(metrics.Metrics{
			metrics.MetricVolumes: &metrics.Metric{
				Created: 1,
				Deleted: 0,
				Errors: 0,
				Running: 2,
			},
		}))

		collector.Delete(metrics.MetricVolumes)
		gomega.Expect(provider.GetMetrics()).To(gomega.Equal(metrics.Metrics{
			metrics.MetricVolumes: &metrics.Metric{
				Created: 1,
				Deleted: 1,
				Errors: 0,
				Running: 1,
			},
		}))

		collector.Error(metrics.MetricVolumes)
		gomega.Expect(provider.GetMetrics()).To(gomega.Equal(metrics.Metrics{
			metrics.MetricVolumes: &metrics.Metric{
				Created: 1,
				Deleted: 1,
				Errors: 1,
				Running: 1,
			},
		}))
	})
})
