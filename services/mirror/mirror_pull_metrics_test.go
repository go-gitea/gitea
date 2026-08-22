// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func getHistogramSampleCount(h *dto.Histogram) uint64 {
	if h == nil {
		return 0
	}
	return h.GetSampleCount()
}

func getHistogramSum(h *dto.Histogram) float64 {
	if h == nil {
		return 0
	}
	return h.GetSampleSum()
}

func collectHistogram(owner, repo, step string) *dto.Histogram {
	observer := mirrorSyncDuration.WithLabelValues(owner, repo, step)
	m := &dto.Metric{}
	if metric, ok := observer.(interface{ Write(*dto.Metric) error }); ok {
		_ = metric.Write(m)
	}
	return m.GetHistogram()
}

func collectCounter(owner, repo, success string) float64 {
	counter := mirrorSyncStatus.WithLabelValues(owner, repo, success)
	m := &dto.Metric{}
	if metric, ok := counter.(interface{ Write(*dto.Metric) error }); ok {
		_ = metric.Write(m)
	}
	return m.GetCounter().GetValue()
}

func TestRecordMirrorSyncStep_DisabledMetrics(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: false, EnabledMirrorSyncDuration: true}

	before := collectHistogram("disabled-org", "disabled-repo", syncSteps.Fetch.label)
	beforeCount := getHistogramSampleCount(before)

	recordMirrorSyncStepWith(cfg, "disabled-org", "disabled-repo", syncSteps.Fetch, time.Now().Add(-100*time.Millisecond))

	after := collectHistogram("disabled-org", "disabled-repo", syncSteps.Fetch.label)
	afterCount := getHistogramSampleCount(after)

	assert.Equal(t, beforeCount, afterCount, "metrics should not be recorded when Enabled is false")
}

func TestRecordMirrorSyncStep_DisabledMirrorSyncDuration(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: false}

	before := collectHistogram("disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch.label)
	beforeCount := getHistogramSampleCount(before)

	recordMirrorSyncStepWith(cfg, "disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch, time.Now().Add(-50*time.Millisecond))

	after := collectHistogram("disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch.label)
	afterCount := getHistogramSampleCount(after)

	assert.Equal(t, beforeCount, afterCount, "metrics should not be recorded when EnabledMirrorSyncDuration is false")
}

func TestRecordMirrorSyncStep_Enabled(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}

	start := time.Now().Add(-200 * time.Millisecond)
	recordMirrorSyncStepWith(cfg, "my-org", "my-repo", syncSteps.Fetch, start)

	h := collectHistogram("my-org", "my-repo", syncSteps.Fetch.label)
	assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1))
	assert.Greater(t, getHistogramSum(h), float64(0))
}

func TestRecordMirrorSyncStep_AllSteps(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}

	steps := []syncStep{
		syncSteps.Fetch,
		syncSteps.CommitGraph,
		syncSteps.LFS,
		syncSteps.Branches,
		syncSteps.Releases,
		syncSteps.RepoSize,
		syncSteps.Wiki,
		syncSteps.Total,
	}

	for _, step := range steps {
		recordMirrorSyncStepWith(cfg, "step-org", "step-repo", step, time.Now().Add(-10*time.Millisecond))
	}

	for _, step := range steps {
		h := collectHistogram("step-org", "step-repo", step.label)
		assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1), "step %q should have at least one observation", step.label)
	}
}

func TestRecordMirrorSyncStatus_Success(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}

	before := collectCounter("status-org", "status-repo", "true")
	recordMirrorSyncStatusWith(cfg, "status-org", "status-repo", true)
	after := collectCounter("status-org", "status-repo", "true")

	assert.Greater(t, after, before)
}

func TestRecordMirrorSyncStatus_Failure(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}

	before := collectCounter("fail-org", "fail-repo", "false")
	recordMirrorSyncStatusWith(cfg, "fail-org", "fail-repo", false)
	after := collectCounter("fail-org", "fail-repo", "false")

	assert.Greater(t, after, before)
}

func TestRecordMirrorSyncStatus_BothStates(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}

	beforeTrue := collectCounter("both-org", "both-repo", "true")
	beforeFalse := collectCounter("both-org", "both-repo", "false")

	recordMirrorSyncStatusWith(cfg, "both-org", "both-repo", true)
	recordMirrorSyncStatusWith(cfg, "both-org", "both-repo", false)

	afterTrue := collectCounter("both-org", "both-repo", "true")
	afterFalse := collectCounter("both-org", "both-repo", "false")

	assert.Greater(t, afterTrue, beforeTrue)
	assert.Greater(t, afterFalse, beforeFalse)
}

func TestRecordMirrorSyncStatus_DisabledMetrics(t *testing.T) {
	cfg := mirrorMetricsConfig{Enabled: false, EnabledMirrorSyncDuration: true}

	before := collectCounter("status-disabled-org", "status-disabled-repo", "true")
	recordMirrorSyncStatusWith(cfg, "status-disabled-org", "status-disabled-repo", true)
	after := collectCounter("status-disabled-org", "status-disabled-repo", "true")

	assert.InDelta(t, before, after, 0, "status metrics should not be recorded when Enabled is false")
}
