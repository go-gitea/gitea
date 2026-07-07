// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/typesniffer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewPullRequestFromRun(t *testing.T) {
	repo := &repo_model.Repository{ID: 1, OwnerName: "owner", Name: "repo"}

	t.Run("pull ref", func(t *testing.T) {
		run := &actions_model.ActionRun{Repo: repo, Ref: "refs/pull/123/head"}
		assert.Equal(t, &ViewPullRequest{Index: "#123", Link: "/owner/repo/pulls/123"}, viewPullRequestFromRun(t.Context(), run, nil))
	})

	t.Run("pull request event payload", func(t *testing.T) {
		// a non-pull ref forces the payload branch instead of the ref branch
		run := &actions_model.ActionRun{Repo: repo, Ref: "refs/heads/feature"}
		payload := &api.PullRequestPayload{Index: 42}
		assert.Equal(t, &ViewPullRequest{Index: "#42", Link: "/owner/repo/pulls/42"}, viewPullRequestFromRun(t.Context(), run, payload))
	})

	t.Run("nil repo", func(t *testing.T) {
		run := &actions_model.ActionRun{Ref: "refs/pull/1/head"}
		assert.Nil(t, viewPullRequestFromRun(t.Context(), run, nil))
	})
}

func TestViewSummaryBranchFromRun(t *testing.T) {
	repo := &repo_model.Repository{ID: 1, OwnerName: "owner", Name: "repo"}

	t.Run("pull request event same repo", func(t *testing.T) {
		run := &actions_model.ActionRun{Repo: repo, Ref: "refs/pull/7/head"}
		payload := &api.PullRequestPayload{
			PullRequest: &api.PullRequest{Head: &api.PRBranchInfo{
				Name:       "feature",
				Ref:        "refs/heads/feature",
				RepoID:     1,
				Repository: &api.Repository{Link: "/owner/repo"},
			}},
		}
		assert.Equal(t, ViewBranch{Name: "feature", Link: "/owner/repo/src/branch/feature"}, viewSummaryBranchFromRun(t.Context(), run, payload))
	})

	t.Run("pull request event from fork prefixes owner", func(t *testing.T) {
		run := &actions_model.ActionRun{Repo: repo, Ref: "refs/pull/7/head"}
		payload := &api.PullRequestPayload{
			PullRequest: &api.PullRequest{Head: &api.PRBranchInfo{
				Name:   "feature",
				Ref:    "refs/heads/feature",
				RepoID: 2,
				Repository: &api.Repository{
					Link:  "/forkowner/repo",
					Owner: &api.User{UserName: "forkowner"},
				},
			}},
		}
		assert.Equal(t, ViewBranch{Name: "forkowner:feature", Link: "/forkowner/repo/src/branch/feature"}, viewSummaryBranchFromRun(t.Context(), run, payload))
	})

	t.Run("push to tag does not query branch", func(t *testing.T) {
		// a tag ref is not a branch, so no GetBranch DB lookup happens
		run := &actions_model.ActionRun{Repo: repo, Ref: "refs/tags/v1.0.0"}
		assert.Equal(t, ViewBranch{Name: "v1.0.0", Link: "/owner/repo/src/tag/v1.0.0"}, viewSummaryBranchFromRun(t.Context(), run, nil))
	})
}

func TestConvertToViewModel(t *testing.T) {
	task := &actions_model.ActionTask{
		Status: actions_model.StatusSuccess,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "Run step-name", Index: 0, Status: actions_model.StatusSuccess, LogLength: 1, Started: timeutil.TimeStamp(1), Stopped: timeutil.TimeStamp(5)},
		},
		Stopped: timeutil.TimeStamp(20),
	}

	viewJobSteps, _, err := convertToViewModel(t.Context(), translation.MockLocale{}, nil, task)
	require.NoError(t, err)

	expectedViewJobs := []*ViewJobStep{
		{
			Summary:  "Set up job",
			Duration: "0s",
			Status:   "success",
		},
		{
			Summary:  "Run step-name",
			Duration: "4s",
			Status:   "success",
		},
		{
			Summary:  "Complete job",
			Duration: "15s",
			Status:   "success",
		},
	}
	assert.Equal(t, expectedViewJobs, viewJobSteps)
}

func resetArtifactPreviewV4ZipListCacheForTest() {
	artifactPreviewV4ZipListCache.mu.Lock()
	defer artifactPreviewV4ZipListCache.mu.Unlock()
	artifactPreviewV4ZipListCache.entries = map[string]artifactPreviewV4ZipListCacheEntry{}
	artifactPreviewV4ZipListCache.order = nil
}

func TestArtifactPreviewV4ZipListCacheSetGet(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	artifact := &actions_model.ActionArtifact{
		ID:          1,
		UpdatedUnix: timeutil.TimeStamp(2),
		StoragePath: "artifact/path.zip",
	}
	paths := []string{"index.html", "logs/output.txt"}
	setArtifactPreviewV4ZipListCache(artifact, paths)

	paths[0] = "changed"
	got, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.True(t, ok)
	assert.Equal(t, []string{"index.html", "logs/output.txt"}, got)

	got[0] = "changed-again"
	got2, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.True(t, ok)
	assert.Equal(t, []string{"index.html", "logs/output.txt"}, got2)
}

func TestArtifactPreviewV4ZipListCacheExpires(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	artifact := &actions_model.ActionArtifact{
		ID:          2,
		UpdatedUnix: timeutil.TimeStamp(3),
		StoragePath: "artifact/expired.zip",
	}
	key := artifactPreviewV4ZipListCacheKey(artifact)

	artifactPreviewV4ZipListCache.mu.Lock()
	artifactPreviewV4ZipListCache.entries[key] = artifactPreviewV4ZipListCacheEntry{
		paths:     []string{"expired.txt"},
		expiresAt: time.Now().Add(-time.Second),
	}
	artifactPreviewV4ZipListCache.order = []string{key}
	artifactPreviewV4ZipListCache.mu.Unlock()

	_, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.False(t, ok)

	artifactPreviewV4ZipListCache.mu.Lock()
	_, exists := artifactPreviewV4ZipListCache.entries[key]
	order := append([]string(nil), artifactPreviewV4ZipListCache.order...)
	artifactPreviewV4ZipListCache.mu.Unlock()
	assert.False(t, exists)
	assert.NotContains(t, order, key)
}

func TestArtifactPreviewV4ZipListCacheEvictsOldest(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	for i := range artifactPreviewV4ZipListCacheMaxEntries + 1 {
		artifact := &actions_model.ActionArtifact{
			ID:          int64(i + 1),
			UpdatedUnix: timeutil.TimeStamp(i + 1),
			StoragePath: "artifact/cache-entry.zip",
		}
		setArtifactPreviewV4ZipListCache(artifact, []string{"file.txt"})
	}

	oldest := &actions_model.ActionArtifact{
		ID:          1,
		UpdatedUnix: timeutil.TimeStamp(1),
		StoragePath: "artifact/cache-entry.zip",
	}
	_, ok := getArtifactPreviewV4ZipListFromCache(oldest)
	assert.False(t, ok)

	newest := &actions_model.ActionArtifact{
		ID:          artifactPreviewV4ZipListCacheMaxEntries + 1,
		UpdatedUnix: timeutil.TimeStamp(artifactPreviewV4ZipListCacheMaxEntries + 1),
		StoragePath: "artifact/cache-entry.zip",
	}
	_, ok = getArtifactPreviewV4ZipListFromCache(newest)
	assert.True(t, ok)
}

func TestLimitArtifactPreviewPathsKeepsSelectedPath(t *testing.T) {
	paths := make([]string, artifactPreviewMaxFiles+10)
	for i := range paths {
		paths[i] = "file-" + strconv.Itoa(i) + ".txt"
	}
	selectedPath := paths[len(paths)-1]

	limited, truncated := limitArtifactPreviewPaths(paths, selectedPath)
	require.True(t, truncated)
	require.Len(t, limited, artifactPreviewMaxFiles)
	assert.Contains(t, limited, selectedPath)
}

func TestNormalizeArtifactPreviewPath(t *testing.T) {
	assert.Empty(t, normalizeArtifactPreviewPath("."))
	assert.Empty(t, normalizeArtifactPreviewPath("./"))
	assert.Equal(t, "report/index.html", normalizeArtifactPreviewPath("./report/index.html"))
}

func TestArtifactPreviewContentTypeUsesPreviewableExtensions(t *testing.T) {
	sniffedText := typesniffer.FromContentType("text/plain; charset=utf-8")

	assert.Equal(t, "text/html; charset=utf-8", artifactPreviewContentType("index.html", sniffedText))
	assert.Equal(t, "text/html; charset=utf-8", artifactPreviewContentType("index.htm", sniffedText))
	assert.Equal(t, "text/css; charset=utf-8", artifactPreviewContentType("style.css", sniffedText))
	assert.Equal(t, "text/plain", artifactPreviewContentType("output.txt", sniffedText))
}

func TestConvertToViewModelCancellingTaskDoesNotRenderRunningSteps(t *testing.T) {
	task := &actions_model.ActionTask{
		Status: actions_model.StatusCancelling,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "Run step-name", Index: 0, Status: actions_model.StatusRunning, LogLength: 1},
		},
	}

	viewJobSteps, _, err := convertToViewModel(t.Context(), translation.MockLocale{}, nil, task)
	require.NoError(t, err)

	expectedViewJobs := []*ViewJobStep{
		{
			Summary:  "Set up job",
			Duration: "0s",
			Status:   "success",
		},
		{
			Summary:  "Run step-name",
			Duration: "0s",
			Status:   "cancelling",
		},
		{
			Summary:  "Complete job",
			Duration: "0s",
			Status:   "waiting",
		},
	}
	assert.Equal(t, expectedViewJobs, viewJobSteps)
}
