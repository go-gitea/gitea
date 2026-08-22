// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"

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

func TestArtifactPreviewV4ZipListCacheKeyChangesOnUpdate(t *testing.T) {
	artifact := &actions_model.ActionArtifact{ID: 1, UpdatedUnix: timeutil.TimeStamp(2)}
	updated := &actions_model.ActionArtifact{ID: 1, UpdatedUnix: timeutil.TimeStamp(3)}
	assert.NotEqual(t, artifactPreviewV4ZipListCacheKey(artifact), artifactPreviewV4ZipListCacheKey(updated))
}

func TestCapArtifactPreviewPaths(t *testing.T) {
	paths := make([]string, artifactPreviewMaxFiles+10)
	for i := range paths {
		paths[i] = "file-" + strconv.Itoa(i) + ".txt"
	}

	capped, truncated := capArtifactPreviewPaths(paths)
	require.True(t, truncated)
	require.Len(t, capped, artifactPreviewMaxFiles)
	// the cap must copy, otherwise the cached slice keeps the full backing array alive
	paths[0] = "changed"
	assert.Equal(t, "file-0.txt", capped[0])

	short := []string{"a.txt", "b.txt"}
	capped, truncated = capArtifactPreviewPaths(short)
	assert.False(t, truncated)
	assert.Equal(t, short, capped)
}

func TestInsertArtifactPreviewPath(t *testing.T) {
	paths := []string{"a.txt", "c.txt", "dir/a.txt"}

	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt", "dir/a.txt"}, insertArtifactPreviewPath(paths, "b.txt"))
	// the source listing may be the cached slice, so it must not be modified in place
	assert.Equal(t, []string{"a.txt", "c.txt", "dir/a.txt"}, paths)
	assert.Equal(t, []string{"a.txt", "c.txt", "dir/a.txt", "z.txt"}, insertArtifactPreviewPath(paths, "z.txt"))
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

func TestPendingNeeds(t *testing.T) {
	current := &actions_model.ActionRunJob{JobID: "deploy", Needs: []string{"build", "test"}}
	jobs := []*actions_model.ActionRunJob{
		current,
		{JobID: "build", Status: actions_model.StatusSuccess},
		{JobID: "test", Status: actions_model.StatusRunning},
	}
	// "test" is not done yet, "build" succeeded, so only "test" blocks.
	assert.Equal(t, []string{"test"}, pendingNeeds(current, jobs))

	t.Run("all needs done", func(t *testing.T) {
		done := []*actions_model.ActionRunJob{
			current,
			{JobID: "build", Status: actions_model.StatusSuccess},
			{JobID: "test", Status: actions_model.StatusSkipped},
		}
		assert.Empty(t, pendingNeeds(current, done))
	})

	t.Run("matrix expansion all required", func(t *testing.T) {
		matrix := []*actions_model.ActionRunJob{
			current,
			{JobID: "build", Status: actions_model.StatusSuccess},
			{JobID: "build", Status: actions_model.StatusRunning},
			{JobID: "test", Status: actions_model.StatusSuccess},
		}
		assert.Equal(t, []string{"build"}, pendingNeeds(current, matrix))
	})

	t.Run("unresolved need treated as pending", func(t *testing.T) {
		missing := []*actions_model.ActionRunJob{current}
		assert.Equal(t, []string{"build", "test"}, pendingNeeds(current, missing))
	})

	t.Run("parent job scope", func(t *testing.T) {
		// a same-named job under a different parent must not satisfy the need
		scoped := &actions_model.ActionRunJob{JobID: "deploy", Needs: []string{"build"}, ParentJobID: 5}
		jobs := []*actions_model.ActionRunJob{
			scoped,
			{JobID: "build", Status: actions_model.StatusSuccess, ParentJobID: 0},
		}
		assert.Equal(t, []string{"build"}, pendingNeeds(scoped, jobs))
	})
}
