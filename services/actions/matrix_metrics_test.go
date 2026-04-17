// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// Essential Prometheus Collector Tests

func TestNewMatrixMetricsCollector(t *testing.T) {
	collector := NewMatrixMetricsCollector()
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.totalReevaluations)
	assert.NotNil(t, collector.successRate)
}

func TestMatrixMetricsCollectorDescribe(t *testing.T) {
	collector := NewMatrixMetricsCollector()
	ch := make(chan *prometheus.Desc, 100)
	collector.Describe(ch)
	assert.NotEmpty(t, ch)
}

func TestMatrixMetricsCollectorCollect(t *testing.T) {
	matrixMetricsInstance = nil
	metrics := GetMatrixMetrics()
	metrics.RecordReevaluation(10*time.Millisecond, true, 5)
	metrics.RecordParseTime(8 * time.Millisecond)

	collector := NewMatrixMetricsCollector()
	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	assert.NotEmpty(t, ch)

	matrixMetricsInstance = nil
}

func TestMatrixMetricsGetStats(t *testing.T) {
	metrics := &MatrixMetrics{
		ReevaluationTimes: make([]time.Duration, 0, 1000),
		ParseTimes:        make([]time.Duration, 0, 1000),
		InsertTimes:       make([]time.Duration, 0, 1000),
	}

	metrics.RecordReevaluation(10*time.Millisecond, true, 3)
	metrics.RecordReevaluation(15*time.Millisecond, true, 2)
	metrics.RecordReevaluation(5*time.Millisecond, false, 0)

	stats := metrics.GetStats()
	assert.Equal(t, int64(3), stats["total_reevaluations"])
	assert.Equal(t, int64(2), stats["successful_reevaluations"])
	assert.Equal(t, int64(1), stats["failed_reevaluations"])
	assert.Greater(t, stats["success_rate_percent"].(float64), 60.0)
}

func BenchmarkMatrixMetricsCollectorCollect(b *testing.B) {
	metrics := &MatrixMetrics{
		ReevaluationTimes: make([]time.Duration, 0, 1000),
		ParseTimes:        make([]time.Duration, 0, 1000),
		InsertTimes:       make([]time.Duration, 0, 1000),
	}
	matrixMetricsInstance = metrics

	for range 100 {
		metrics.RecordReevaluation(10*time.Millisecond, true, 5)
		metrics.RecordParseTime(5 * time.Millisecond)
	}

	collector := NewMatrixMetricsCollector()

	b.ResetTimer()
	for b.Loop() {
		// Use a fresh channel each iteration to avoid filling up the buffer
		ch := make(chan prometheus.Metric, 100)
		collector.Collect(ch)
		// Drain the channel to prevent blocking
		close(ch)
		for range ch {
			// discard metrics
		}
	}
}
