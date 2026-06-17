// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"
	"time"

	"gitea.dev/modules/setting"

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

func collectStatusHistogram(owner, repo, success string) *dto.Histogram {
	observer := mirrorSyncStatus.WithLabelValues(owner, repo, success)
	m := &dto.Metric{}
	if metric, ok := observer.(interface{ Write(*dto.Metric) error }); ok {
		_ = metric.Write(m)
	}
	return m.GetHistogram()
}

func TestRecordMirrorSyncStep_DisabledMetrics(t *testing.T) {
	setting.Metrics.Enabled = false
	setting.Metrics.EnabledMirrorSyncDuration = true

	before := collectHistogram("disabled-org", "disabled-repo", syncSteps.Fetch.label)
	beforeCount := getHistogramSampleCount(before)

	recordMirrorSyncStep("disabled-org", "disabled-repo", syncSteps.Fetch, time.Now().Add(-100*time.Millisecond))

	after := collectHistogram("disabled-org", "disabled-repo", syncSteps.Fetch.label)
	afterCount := getHistogramSampleCount(after)

	assert.Equal(t, beforeCount, afterCount, "metrics should not be recorded when Metrics.Enabled is false")
}

func TestRecordMirrorSyncStep_DisabledMirrorSyncDuration(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = false

	before := collectHistogram("disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch.label)
	beforeCount := getHistogramSampleCount(before)

	recordMirrorSyncStep("disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch, time.Now().Add(-50*time.Millisecond))

	after := collectHistogram("disabled-mirror-org", "disabled-mirror-repo", syncSteps.Fetch.label)
	afterCount := getHistogramSampleCount(after)

	assert.Equal(t, beforeCount, afterCount, "metrics should not be recorded when EnabledMirrorSyncDuration is false")
}

func TestRecordMirrorSyncStep_Enabled(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

	start := time.Now().Add(-200 * time.Millisecond)
	recordMirrorSyncStep("my-org", "my-repo", syncSteps.Fetch, start)

	// Prometheus metric name: gitea_mirror_sync_duration_ms
	// With labels: {owner="my-org", repo="my-repo", step="fetch"}
	h := collectHistogram("my-org", "my-repo", syncSteps.Fetch.label)
	assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1))
	assert.Greater(t, getHistogramSum(h), float64(0))
}

func TestRecordMirrorSyncStep_AllSteps(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

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
		recordMirrorSyncStep("step-org", "step-repo", step, time.Now().Add(-10*time.Millisecond))
	}

	// Verify all steps recorded: gitea_mirror_sync_duration_ms_bucket, gitea_mirror_sync_duration_ms_sum, gitea_mirror_sync_duration_ms_count
	for _, step := range steps {
		h := collectHistogram("step-org", "step-repo", step.label)
		assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1), "step %q should have at least one observation", step.label)
	}
}

func TestRecordMirrorSyncStatus_Success(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

	recordMirrorSyncStatus("status-org", "status-repo", true)

	// Prometheus metric name: gitea_mirror_sync_status
	// With labels: {owner="status-org", repo="status-repo", success="true"}
	h := collectStatusHistogram("status-org", "status-repo", "true")
	assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1))
}

func TestRecordMirrorSyncStatus_Failure(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

	recordMirrorSyncStatus("fail-org", "fail-repo", false)

	// Prometheus metric name: gitea_mirror_sync_status
	// With labels: {owner="fail-org", repo="fail-repo", success="false"}
	h := collectStatusHistogram("fail-org", "fail-repo", "false")
	assert.GreaterOrEqual(t, getHistogramSampleCount(h), uint64(1))
}

func TestRecordMirrorSyncStatus_BothStates(t *testing.T) {
	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

	recordMirrorSyncStatus("both-org", "both-repo", true)
	recordMirrorSyncStatus("both-org", "both-repo", false)

	// gitea_mirror_sync_status_count{owner="both-org", repo="both-repo", success="true"} == 1
	hTrue := collectStatusHistogram("both-org", "both-repo", "true")
	assert.GreaterOrEqual(t, getHistogramSampleCount(hTrue), uint64(1))

	// gitea_mirror_sync_status_count{owner="both-org", repo="both-repo", success="false"} == 1
	hFalse := collectStatusHistogram("both-org", "both-repo", "false")
	assert.GreaterOrEqual(t, getHistogramSampleCount(hFalse), uint64(1))
}

func TestRecordMirrorSyncStatus_DisabledMetrics(t *testing.T) {
	setting.Metrics.Enabled = false
	setting.Metrics.EnabledMirrorSyncDuration = true

	before := collectStatusHistogram("status-disabled-org", "status-disabled-repo", "true")
	beforeCount := getHistogramSampleCount(before)

	recordMirrorSyncStatus("status-disabled-org", "status-disabled-repo", true)

	after := collectStatusHistogram("status-disabled-org", "status-disabled-repo", "true")
	afterCount := getHistogramSampleCount(after)

	assert.Equal(t, beforeCount, afterCount, "status metrics should not be recorded when Metrics.Enabled is false")
}
