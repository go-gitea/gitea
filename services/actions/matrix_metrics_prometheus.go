// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MatrixMetricsCollector implements the prometheus.Collector interface
// and exposes matrix re-evaluation metrics for prometheus
type MatrixMetricsCollector struct {
	// Counters
	totalReevaluations      prometheus.Gauge
	successfulReevaluations prometheus.Gauge
	failedReevaluations     prometheus.Gauge
	deferredReevaluations   prometheus.Gauge
	jobsCreatedTotal        prometheus.Gauge

	// Timing (in milliseconds)
	totalReevaluationTime prometheus.Gauge
	avgReevaluationTime   prometheus.Gauge
	totalParseTime        prometheus.Gauge
	avgParseTime          prometheus.Gauge
	totalInsertTime       prometheus.Gauge
	avgInsertTime         prometheus.Gauge

	// Rates
	successRate prometheus.Gauge
}

const (
	namespace = "gitea_"
	subsystem = "matrix_"
)

// NewMatrixMetricsCollector creates a new MatrixMetricsCollector
func NewMatrixMetricsCollector() *MatrixMetricsCollector {
	return &MatrixMetricsCollector{
		totalReevaluations: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "total_reevaluations",
				Help:      "Total number of matrix re-evaluation attempts",
			},
		),
		successfulReevaluations: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "successful_reevaluations",
				Help:      "Number of successful matrix re-evaluations",
			},
		),
		failedReevaluations: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "failed_reevaluations",
				Help:      "Number of failed matrix re-evaluations",
			},
		),
		deferredReevaluations: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "deferred_reevaluations",
				Help:      "Number of deferred matrix re-evaluations (waiting for dependencies)",
			},
		),
		jobsCreatedTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "jobs_created_total",
				Help:      "Total number of jobs created from matrix expansion",
			},
		),
		totalReevaluationTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "total_reevaluation_time_ms",
				Help:      "Total time spent on matrix re-evaluations in milliseconds",
			},
		),
		avgReevaluationTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "avg_reevaluation_time_ms",
				Help:      "Average time per matrix re-evaluation in milliseconds",
			},
		),
		totalParseTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "total_parse_time_ms",
				Help:      "Total time spent parsing workflow payloads in milliseconds",
			},
		),
		avgParseTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "avg_parse_time_ms",
				Help:      "Average time per workflow parse in milliseconds",
			},
		),
		totalInsertTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "total_insert_time_ms",
				Help:      "Total time spent inserting jobs into database in milliseconds",
			},
		),
		avgInsertTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "avg_insert_time_ms",
				Help:      "Average time per database insert in milliseconds",
			},
		),
		successRate: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "success_rate_percent",
				Help:      "Success rate of matrix re-evaluations as percentage (0-100)",
			},
		),
	}
}

// Describe returns the metrics descriptions
func (c *MatrixMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.totalReevaluations.Describe(ch)
	c.successfulReevaluations.Describe(ch)
	c.failedReevaluations.Describe(ch)
	c.deferredReevaluations.Describe(ch)
	c.jobsCreatedTotal.Describe(ch)
	c.totalReevaluationTime.Describe(ch)
	c.avgReevaluationTime.Describe(ch)
	c.totalParseTime.Describe(ch)
	c.avgParseTime.Describe(ch)
	c.totalInsertTime.Describe(ch)
	c.avgInsertTime.Describe(ch)
	c.successRate.Describe(ch)
}

// Collect collects the current metric values and sends them to the channel
func (c *MatrixMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	metrics := GetMatrixMetrics()
	stats := metrics.GetStats()

	// Set counter values
	c.totalReevaluations.Set(float64(stats["total_reevaluations"].(int64)))
	c.successfulReevaluations.Set(float64(stats["successful_reevaluations"].(int64)))
	c.failedReevaluations.Set(float64(stats["failed_reevaluations"].(int64)))
	c.deferredReevaluations.Set(float64(stats["deferred_reevaluations"].(int64)))
	c.jobsCreatedTotal.Set(float64(stats["total_jobs_created"].(int64)))

	// Set timing values (already in milliseconds)
	c.totalReevaluationTime.Set(float64(stats["total_reevaluation_time_ms"].(int64)))
	c.avgReevaluationTime.Set(float64(stats["avg_reevaluation_time_ms"].(int64)))
	c.totalParseTime.Set(float64(stats["total_parse_time_ms"].(int64)))
	c.avgParseTime.Set(float64(stats["avg_parse_time_ms"].(int64)))
	c.totalInsertTime.Set(float64(stats["total_insert_time_ms"].(int64)))
	c.avgInsertTime.Set(float64(stats["avg_insert_time_ms"].(int64)))

	// Set success rate
	c.successRate.Set(stats["success_rate_percent"].(float64))

	// Collect all metrics
	c.totalReevaluations.Collect(ch)
	c.successfulReevaluations.Collect(ch)
	c.failedReevaluations.Collect(ch)
	c.deferredReevaluations.Collect(ch)
	c.jobsCreatedTotal.Collect(ch)
	c.totalReevaluationTime.Collect(ch)
	c.avgReevaluationTime.Collect(ch)
	c.totalParseTime.Collect(ch)
	c.avgParseTime.Collect(ch)
	c.totalInsertTime.Collect(ch)
	c.avgInsertTime.Collect(ch)
	c.successRate.Collect(ch)
}
