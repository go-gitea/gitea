// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"strconv"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

var mirrorSyncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "gitea",
	Subsystem: "mirror",
	Name:      "sync_duration_ms",
	Help:      "Duration in milliseconds of mirror pull sync steps.",
	Buckets:   []float64{5, 1000, 5000, 10000, 30000, 60000, 300000},
}, []string{"owner", "repo", "step"})

var mirrorSyncStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gitea",
	Subsystem: "mirror",
	Name:      "sync_total",
	Help:      "Count of mirror pull sync completions, labeled by success/failure.",
}, []string{"owner", "repo", "success"})

func init() {
	prometheus.MustRegister(mirrorSyncDuration, mirrorSyncStatus)
}

type mirrorMetricsConfig struct {
	Enabled                   bool
	EnabledMirrorSyncDuration bool
}

func defaultMetricsConfig() mirrorMetricsConfig {
	return mirrorMetricsConfig{
		Enabled:                   setting.Metrics.Enabled,
		EnabledMirrorSyncDuration: setting.Metrics.EnabledMirrorSyncDuration,
	}
}

func recordMirrorSyncStep(owner, repo, step string, start time.Time) {
	recordMirrorSyncStepWith(defaultMetricsConfig(), owner, repo, step, start)
}

func recordMirrorSyncStepWith(cfg mirrorMetricsConfig, owner, repo, step string, start time.Time) {
	if !cfg.Enabled || !cfg.EnabledMirrorSyncDuration {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic in recordMirrorSyncStep: %v", r)
		}
	}()
	mirrorSyncDuration.WithLabelValues(owner, repo, step).Observe(float64(time.Since(start).Milliseconds()))
}

func recordMirrorSyncStatus(owner, repo string, success bool) {
	recordMirrorSyncStatusWith(defaultMetricsConfig(), owner, repo, success)
}

func recordMirrorSyncStatusWith(cfg mirrorMetricsConfig, owner, repo string, success bool) {
	if !cfg.Enabled || !cfg.EnabledMirrorSyncDuration {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic in recordMirrorSyncStatus: %v", r)
		}
	}()
	mirrorSyncStatus.WithLabelValues(owner, repo, strconv.FormatBool(success)).Inc()
}
