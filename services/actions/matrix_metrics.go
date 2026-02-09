// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"sync"
	"time"
)

// MatrixMetrics tracks performance metrics for matrix re-evaluation operations
type MatrixMetrics struct {
	mu sync.RWMutex

	// Counters
	TotalReevaluations      int64
	SuccessfulReevaluations int64
	FailedReevaluations     int64
	JobsCreatedTotal        int64
	DeferredReevaluations   int64

	// Timing
	TotalReevaluationTime time.Duration
	TotalParseTime        time.Duration
	TotalInsertTime       time.Duration

	// Histograms (for detailed analysis)
	ReevaluationTimes []time.Duration
	ParseTimes        []time.Duration
	InsertTimes       []time.Duration
}

var (
	matrixMetricsInstance *MatrixMetrics
	metricsMutex          sync.Mutex
)

// GetMatrixMetrics returns the global matrix metrics instance
func GetMatrixMetrics() *MatrixMetrics {
	if matrixMetricsInstance == nil {
		metricsMutex.Lock()
		if matrixMetricsInstance == nil {
			matrixMetricsInstance = &MatrixMetrics{
				ReevaluationTimes: make([]time.Duration, 0, 1000),
				ParseTimes:        make([]time.Duration, 0, 1000),
				InsertTimes:       make([]time.Duration, 0, 1000),
			}
		}
		metricsMutex.Unlock()
	}
	return matrixMetricsInstance
}

// appendToHistogram appends a duration to a histogram with rolling window (keep last 1000)
func appendToHistogram(histogram *[]time.Duration, duration time.Duration) {
	if len(*histogram) < 1000 {
		*histogram = append(*histogram, duration)
	} else {
		// Shift and add new value
		copy(*histogram, (*histogram)[1:])
		(*histogram)[len(*histogram)-1] = duration
	}
}

// RecordReevaluation records a matrix re-evaluation attempt
func (m *MatrixMetrics) RecordReevaluation(duration time.Duration, success bool, jobsCreated int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalReevaluations++
	m.TotalReevaluationTime += duration

	if success {
		m.SuccessfulReevaluations++
		m.JobsCreatedTotal += jobsCreated
	} else {
		m.FailedReevaluations++
	}

	appendToHistogram(&m.ReevaluationTimes, duration)
}

// RecordDeferred records a deferred matrix re-evaluation
func (m *MatrixMetrics) RecordDeferred() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeferredReevaluations++
}

// RecordParseTime records the time taken to parse a workflow
func (m *MatrixMetrics) RecordParseTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalParseTime += duration
	appendToHistogram(&m.ParseTimes, duration)
}

// RecordInsertTime records the time taken to insert matrix jobs
func (m *MatrixMetrics) RecordInsertTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalInsertTime += duration
	appendToHistogram(&m.InsertTimes, duration)
}

// GetStats returns a snapshot of the current metrics
func (m *MatrixMetrics) GetStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgReevaluationTime := time.Duration(0)
	if m.TotalReevaluations > 0 {
		avgReevaluationTime = m.TotalReevaluationTime / time.Duration(m.TotalReevaluations)
	}

	avgParseTime := time.Duration(0)
	if len(m.ParseTimes) > 0 {
		avgParseTime = m.TotalParseTime / time.Duration(len(m.ParseTimes))
	}

	avgInsertTime := time.Duration(0)
	if len(m.InsertTimes) > 0 {
		avgInsertTime = m.TotalInsertTime / time.Duration(len(m.InsertTimes))
	}

	successRate := 0.0
	if m.TotalReevaluations > 0 {
		successRate = float64(m.SuccessfulReevaluations) / float64(m.TotalReevaluations) * 100
	}

	return map[string]any{
		"total_reevaluations":        m.TotalReevaluations,
		"successful_reevaluations":   m.SuccessfulReevaluations,
		"failed_reevaluations":       m.FailedReevaluations,
		"deferred_reevaluations":     m.DeferredReevaluations,
		"success_rate_percent":       successRate,
		"total_jobs_created":         m.JobsCreatedTotal,
		"total_reevaluation_time_ms": m.TotalReevaluationTime.Milliseconds(),
		"avg_reevaluation_time_ms":   avgReevaluationTime.Milliseconds(),
		"total_parse_time_ms":        m.TotalParseTime.Milliseconds(),
		"avg_parse_time_ms":          avgParseTime.Milliseconds(),
		"total_insert_time_ms":       m.TotalInsertTime.Milliseconds(),
		"avg_insert_time_ms":         avgInsertTime.Milliseconds(),
	}
}

// Reset clears all metrics
func (m *MatrixMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalReevaluations = 0
	m.SuccessfulReevaluations = 0
	m.FailedReevaluations = 0
	m.JobsCreatedTotal = 0
	m.DeferredReevaluations = 0
	m.TotalReevaluationTime = 0
	m.TotalParseTime = 0
	m.TotalInsertTime = 0
	m.ReevaluationTimes = m.ReevaluationTimes[:0]
	m.ParseTimes = m.ParseTimes[:0]
	m.InsertTimes = m.InsertTimes[:0]
}
