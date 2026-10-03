// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

type syncStep struct{ label string }

var syncSteps = struct {
	Fetch, CommitGraph, LFS, Branches, Releases, RepoSize, Wiki, Total syncStep
}{
	Fetch:       syncStep{"fetch"},
	CommitGraph: syncStep{"commit_graph"},
	LFS:         syncStep{"lfs"},
	Branches:    syncStep{"branches"},
	Releases:    syncStep{"releases"},
	RepoSize:    syncStep{"repo_size"},
	Wiki:        syncStep{"wiki"},
	Total:       syncStep{"total"},
}

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
	prometheus.MustRegister(mirrorSyncDuration)
	prometheus.MustRegister(mirrorSyncStatus)
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

func recordMirrorSyncStep(owner, repo string, step syncStep, start time.Time) {
	recordMirrorSyncStepWith(defaultMetricsConfig(), owner, repo, step, start)
}

func recordMirrorSyncStepWith(cfg mirrorMetricsConfig, owner, repo string, step syncStep, start time.Time) {
	if !cfg.Enabled || !cfg.EnabledMirrorSyncDuration {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic in recordMirrorSyncStep: %v", r)
		}
	}()
	elapsed := float64(time.Since(start).Milliseconds())
	mirrorSyncDuration.WithLabelValues(owner, repo, step.label).Observe(elapsed)
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
	s := "false"
	if success {
		s = "true"
	}
	mirrorSyncStatus.WithLabelValues(owner, repo, s).Inc()
}
