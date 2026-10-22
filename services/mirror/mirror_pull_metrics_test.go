// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectMirrorSyncMetric(t *testing.T, metric prometheus.Metric) *dto.Metric {
	t.Helper()
	result := &dto.Metric{}
	require.NoError(t, metric.Write(result))
	return result
}

func TestRecordMirrorSyncMetrics(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		cfg   mirrorMetricsConfig
		count uint64
	}{
		{"enabled", mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: true}, 1},
		{"metrics_disabled", mirrorMetricsConfig{Enabled: false, EnabledMirrorSyncDuration: true}, 0},
		{"duration_disabled", mirrorMetricsConfig{Enabled: true, EnabledMirrorSyncDuration: false}, 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			histogram, ok := mirrorSyncDuration.WithLabelValues("org", t.Name(), "fetch").(prometheus.Metric)
			require.True(t, ok)
			before := collectMirrorSyncMetric(t, histogram).GetHistogram()
			recordMirrorSyncStepWith(testCase.cfg, "org", t.Name(), "fetch", time.Now().Add(-200*time.Millisecond))
			after := collectMirrorSyncMetric(t, histogram).GetHistogram()
			assert.Equal(t, before.GetSampleCount()+testCase.count, after.GetSampleCount())
			if testCase.count == 0 {
				assert.InDelta(t, before.GetSampleSum(), after.GetSampleSum(), 0)
			} else {
				assert.GreaterOrEqual(t, after.GetSampleSum()-before.GetSampleSum(), float64(200))
			}

			for _, state := range []struct {
				success bool
				label   string
			}{
				{true, "true"},
				{false, "false"},
			} {
				t.Run(state.label, func(t *testing.T) {
					counter := mirrorSyncStatus.WithLabelValues("org", t.Name(), state.label)
					before := collectMirrorSyncMetric(t, counter).GetCounter().GetValue()
					recordMirrorSyncStatusWith(testCase.cfg, "org", t.Name(), state.success)
					assert.InDelta(t, before+float64(testCase.count), collectMirrorSyncMetric(t, counter).GetCounter().GetValue(), 0)
				})
			}
		})
	}
}
