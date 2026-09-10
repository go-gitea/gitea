// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

var mirrorLFSPendingGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gitea",
	Subsystem: "mirror",
	Name:      "lfs_pending_objects",
	Help:      "Current number of LFS objects pending download from upstream.",
}, []string{"owner", "repo"})

func init() {
	prometheus.MustRegister(mirrorLFSPendingGauge)
}

func recordMirrorLFSPending(owner, repo string, count int64) {
	if !setting.Metrics.Enabled || !setting.Metrics.EnabledMirrorSyncDuration {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("mirror.recordMirrorLFSPending panic: %v", r)
		}
	}()
	mirrorLFSPendingGauge.WithLabelValues(owner, repo).Set(float64(count))
}
