// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

var repoRefCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gitea",
	Name:      "ref_count",
	Help:      "Number of git refs per repository, labeled by ref storage type (packed or loose).",
}, []string{"owner", "repo", "type"})

var repoRefCountCollectionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: "gitea",
	Name:      "ref_count_collection_duration_ms",
	Help:      "Duration in milliseconds to collect ref_count metrics across all repositories.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	prometheus.MustRegister(repoRefCount)
	prometheus.MustRegister(repoRefCountCollectionDuration)
}

type refMetricsConfig struct {
	Enabled         bool
	EnabledRefCount bool
}

// CollectRepoRefCounts walks every repository on disk and updates the
// gitea_ref_count gauge with packed and loose ref counts per repo.
func CollectRepoRefCounts(ctx context.Context) {
	collectRepoRefCountsWith(ctx, refMetricsConfig{
		Enabled:         setting.Metrics.Enabled,
		EnabledRefCount: setting.Metrics.EnabledRefCount,
	})
}

func collectRepoRefCountsWith(ctx context.Context, cfg refMetricsConfig) {
	if !cfg.Enabled || !cfg.EnabledRefCount {
		return
	}

	start := time.Now()
	defer func() {
		repoRefCountCollectionDuration.Observe(float64(time.Since(start).Milliseconds()))
	}()

	if err := db.Iterate[repo_model.Repository](ctx, nil, func(ctx context.Context, repo *repo_model.Repository) error {
		repoPath := gitrepo.RepoLocalPath(repo)
		packed, loose := countRefs(repoPath)
		repoRefCount.WithLabelValues(repo.OwnerName, repo.Name, "packed").Set(float64(packed))
		repoRefCount.WithLabelValues(repo.OwnerName, repo.Name, "loose").Set(float64(loose))
		return nil
	}); err != nil {
		log.Error("CollectRepoRefCounts: failed to iterate repositories: %v", err)
	}
}

// countRefs returns the number of packed refs (from packed-refs) and loose refs
// (files under refs/) for the bare repository at repoPath.
func countRefs(repoPath string) (packed, loose int) {
	packed = countPackedRefs(filepath.Join(repoPath, "packed-refs"))
	loose = countLooseRefs(filepath.Join(repoPath, "refs"))
	return packed, loose
}

// countPackedRefs counts lines in packed-refs that begin with a hex SHA
// (skipping the header line and peeled-tag '^' lines).
func countPackedRefs(packedRefsPath string) int {
	f, err := os.Open(packedRefsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("countPackedRefs: cannot open %s: %v", packedRefsPath, err)
		}
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && (line[0] >= '0' && line[0] <= '9' || line[0] >= 'a' && line[0] <= 'f') {
			count++
		}
	}
	return count
}

// countLooseRefs counts regular files under the refs/ directory tree.
func countLooseRefs(refsDir string) int {
	count := 0
	_ = filepath.WalkDir(refsDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}
