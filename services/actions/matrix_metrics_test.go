// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMatrixMetrics_DeferredCheckIncrementsCounter(t *testing.T) {
	before := testutil.ToFloat64(matrixReevaluations.WithLabelValues("deferred"))
	observeDeferredCheck()
	observeDeferredCheck()
	after := testutil.ToFloat64(matrixReevaluations.WithLabelValues("deferred"))
	assert.InDelta(t, before+2, after, 0)
}

func TestMatrixMetrics_TimingsEndRecordsOutcome(t *testing.T) {
	beforeSuccess := testutil.ToFloat64(matrixReevaluations.WithLabelValues("success"))
	beforeJobs := testutil.ToFloat64(matrixJobsCreated)

	timings := newMatrixTimings()
	// Pretend the parse + insert phases took some time. The histograms are
	// observed via the start/end pair; ToFloat64 doesn't read histogram bucket
	// counts but we can at least exercise the methods without panicking.
	timings.startParse()
	time.Sleep(1 * time.Millisecond)
	timings.endParse()
	timings.startInsert()
	time.Sleep(1 * time.Millisecond)
	timings.endInsert()
	timings.end("success", 4)

	afterSuccess := testutil.ToFloat64(matrixReevaluations.WithLabelValues("success"))
	afterJobs := testutil.ToFloat64(matrixJobsCreated)
	assert.InDelta(t, beforeSuccess+1, afterSuccess, 0, "success counter must tick once per end(success, ...)")
	assert.InDelta(t, beforeJobs+4, afterJobs, 0, "jobs counter must tick by childrenCount on success")
}

func TestMatrixMetrics_TimingsFailureDoesNotAddJobs(t *testing.T) {
	beforeFailure := testutil.ToFloat64(matrixReevaluations.WithLabelValues("failure"))
	beforeJobs := testutil.ToFloat64(matrixJobsCreated)

	timings := newMatrixTimings()
	timings.end("failure", 99) // childrenCount is ignored on non-success

	afterFailure := testutil.ToFloat64(matrixReevaluations.WithLabelValues("failure"))
	afterJobs := testutil.ToFloat64(matrixJobsCreated)
	assert.InDelta(t, beforeFailure+1, afterFailure, 0)
	assert.InDelta(t, beforeJobs, afterJobs, 0, "failures must not pollute the jobs-created counter")
}
