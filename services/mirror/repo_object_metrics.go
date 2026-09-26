// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"bufio"
	"bytes"
	"context"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

var repoObjectCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gitea",
	Name:      "object_count",
	Help:      "Number of git objects per repository, labeled by object type (blob, commit, tree).",
}, []string{"owner", "repo", "type"})

var repoObjectCountCollectionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: "gitea",
	Name:      "object_count_collection_duration_ms",
	Help:      "Duration in milliseconds to collect object_count metrics across all repositories.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	prometheus.MustRegister(repoObjectCount)
	prometheus.MustRegister(repoObjectCountCollectionDuration)
}

type objectMetricsConfig struct {
	Enabled            bool
	EnabledObjectCount bool
}

// CollectRepoObjectCounts walks every repository and updates the gitea_object_count
// gauge with blob, commit, and tree counts per repo using git cat-file --batch-check.
func CollectRepoObjectCounts(ctx context.Context) {
	collectRepoObjectCountsWith(ctx, objectMetricsConfig{
		Enabled:            setting.Metrics.Enabled,
		EnabledObjectCount: setting.Metrics.EnabledObjectCount,
	})
}

func collectRepoObjectCountsWith(ctx context.Context, cfg objectMetricsConfig) {
	if !cfg.Enabled || !cfg.EnabledObjectCount {
		return
	}

	start := time.Now()
	defer func() {
		repoObjectCountCollectionDuration.Observe(float64(time.Since(start).Milliseconds()))
	}()

	if err := db.Iterate[repo_model.Repository](ctx, nil, func(ctx context.Context, repo *repo_model.Repository) error {
		blobs, commits, trees := countObjects(ctx, repo)
		repoObjectCount.WithLabelValues(repo.OwnerName, repo.Name, "blob").Set(float64(blobs))
		repoObjectCount.WithLabelValues(repo.OwnerName, repo.Name, "commit").Set(float64(commits))
		repoObjectCount.WithLabelValues(repo.OwnerName, repo.Name, "tree").Set(float64(trees))
		return nil
	}); err != nil {
		log.Error("CollectRepoObjectCounts: failed to iterate repositories: %v", err)
	}
}

// countObjects runs git cat-file --batch-check --batch-all-objects and counts
// blob, commit, and tree objects for the bare repository.
func countObjects(ctx context.Context, repo gitrepo.RepositoryFacade) (blobs, commits, trees int) {
	stdout, stderr, err := gitcmd.NewCommand("cat-file", "--batch-check", "--batch-all-objects").
		WithRepo(repo).RunStdBytes(ctx)
	if err != nil {
		log.Warn("countObjects: cat-file failed for %s: %v - %s", repo.LogString(), err, string(stderr))
		return 0, 0, 0
	}

	var buf bytes.Buffer
	buf.Write(stdout)

	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		fields := bytes.Fields(scanner.Bytes())
		if len(fields) < 2 {
			continue
		}
		switch string(fields[1]) {
		case "blob":
			blobs++
		case "commit":
			commits++
		case "tree":
			trees++
		}
	}
	return blobs, commits, trees
}
