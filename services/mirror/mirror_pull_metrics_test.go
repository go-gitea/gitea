// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	"gitea.dev/modules/setting"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getGaugeValue(t *testing.T, owner, repo string) float64 {
	t.Helper()
	gauge, err := mirrorLFSPendingGauge.GetMetricWithLabelValues(owner, repo)
	require.NoError(t, err)
	var m dto.Metric
	require.NoError(t, gauge.(interface{ Write(*dto.Metric) error }).Write(&m))
	return m.GetGauge().GetValue()
}

func TestRecordMirrorLFSPendingDisabledWhenMetricsOff(t *testing.T) {
	orig := setting.Metrics
	defer func() { setting.Metrics = orig }()

	setting.Metrics.Enabled = false
	setting.Metrics.EnabledMirrorSyncDuration = true

	recordMirrorLFSPending("org", "repo", 42)

	assert.InDelta(t, float64(0), getGaugeValue(t, "org", "repo"), 1e-9)
}

func TestRecordMirrorLFSPendingDisabledWhenDurationOff(t *testing.T) {
	orig := setting.Metrics
	defer func() { setting.Metrics = orig }()

	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = false

	recordMirrorLFSPending("org2", "repo2", 7)

	assert.InDelta(t, float64(0), getGaugeValue(t, "org2", "repo2"), 1e-9)
}

func TestRecordMirrorLFSPendingEnabled(t *testing.T) {
	orig := setting.Metrics
	defer func() { setting.Metrics = orig }()

	setting.Metrics.Enabled = true
	setting.Metrics.EnabledMirrorSyncDuration = true

	recordMirrorLFSPending("enabled-org", "enabled-repo", 5)
	assert.InDelta(t, float64(5), getGaugeValue(t, "enabled-org", "enabled-repo"), 1e-9)

	recordMirrorLFSPending("enabled-org", "enabled-repo", 3)
	assert.InDelta(t, float64(3), getGaugeValue(t, "enabled-org", "enabled-repo"), 1e-9)
}
