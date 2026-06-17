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

var mirrorSyncStatus = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "gitea",
	Subsystem: "mirror",
	Name:      "sync_status",
	Help:      "Count of mirror pull sync completions, labeled by success/failure.",
	Buckets:   []float64{0, 1},
}, []string{"owner", "repo", "success"})

func init() {
	prometheus.MustRegister(mirrorSyncDuration)
	prometheus.MustRegister(mirrorSyncStatus)
}

func recordMirrorSyncStep(owner, repo string, step syncStep, start time.Time) {
	if !setting.Metrics.Enabled || !setting.Metrics.EnabledMirrorSyncDuration {
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
	if !setting.Metrics.Enabled || !setting.Metrics.EnabledMirrorSyncDuration {
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
	mirrorSyncStatus.WithLabelValues(owner, repo, s).Observe(1)
}
