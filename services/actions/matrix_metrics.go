// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus instrumentation for the deferred-matrix expansion pipeline.
//
// Counters and histograms are registered unconditionally on package init so
// the same /metrics endpoint Gitea already serves (when [metrics].ENABLED is
// set) exposes them with no extra wiring. The counters add a few atomic
// increments per expansion — negligible overhead when nobody scrapes.
var (
	matrixReevaluations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitea_matrix_reevaluations_total",
			Help: "Deferred-matrix re-evaluation attempts, partitioned by outcome (success, failure, deferred).",
		},
		[]string{"outcome"},
	)
	matrixJobsCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gitea_matrix_jobs_created_total",
			Help: "Child RunJobs created by deferred-matrix expansion.",
		},
	)
	matrixReevaluationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gitea_matrix_reevaluation_duration_seconds",
			Help:    "End-to-end wall-clock time per deferred-matrix expansion.",
			Buckets: prometheus.DefBuckets,
		},
	)
	matrixParseDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gitea_matrix_parse_duration_seconds",
			Help:    "Wall-clock time re-parsing the placeholder WorkflowPayload and evaluating the matrix expression.",
			Buckets: prometheus.DefBuckets,
		},
	)
	matrixInsertDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gitea_matrix_insert_duration_seconds",
			Help:    "Wall-clock time inserting expanded child RunJobs into the database.",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(
		matrixReevaluations,
		matrixJobsCreated,
		matrixReevaluationDuration,
		matrixParseDuration,
		matrixInsertDuration,
	)
}

// matrixTimings tracks the per-expansion stopwatch so the histograms get
// consistent observations. Use newMatrixTimings → startParse/endParse →
// startInsert/endInsert → end(outcome, count).
type matrixTimings struct {
	overall time.Time
	parse   time.Time
	insert  time.Time
}

func newMatrixTimings() *matrixTimings {
	return &matrixTimings{overall: time.Now()}
}

func (t *matrixTimings) startParse()  { t.parse = time.Now() }
func (t *matrixTimings) endParse()    { matrixParseDuration.Observe(time.Since(t.parse).Seconds()) }
func (t *matrixTimings) startInsert() { t.insert = time.Now() }
func (t *matrixTimings) endInsert()   { matrixInsertDuration.Observe(time.Since(t.insert).Seconds()) }

// end finalises the per-expansion observation. outcome ∈ {success, failure, deferred}.
// childrenCount is only added for outcome="success".
func (t *matrixTimings) end(outcome string, childrenCount int) {
	matrixReevaluations.WithLabelValues(outcome).Inc()
	if outcome == "success" && childrenCount > 0 {
		matrixJobsCreated.Add(float64(childrenCount))
	}
	matrixReevaluationDuration.Observe(time.Since(t.overall).Seconds())
}

// observeDeferred records a placeholder check that found needs not yet
// satisfied. No histogram observation here — the check is cheap and
// otherwise floods the duration distribution with sub-millisecond samples.
func observeDeferredCheck() {
	matrixReevaluations.WithLabelValues("deferred").Inc()
}
