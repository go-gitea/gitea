// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateReleaseNotes(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)

	mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
	pr := createMergedPullRequest(t, repo, mergedCommit, 5)

	result, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
		TagName: "v1.2.0",
		Target:  "DefaultBranch",
	})
	require.NoError(t, err)

	assert.Equal(t, "v1.1", result.PreviousTag)
	assert.Contains(t, result.Content, "## What's Changed")
	prURL := pr.Issue.HTMLURL(t.Context())
	assert.Contains(t, result.Content, fmt.Sprintf("%s in [#%d](%s)", pr.Issue.Title, pr.Index, prURL))
	assert.Contains(t, result.Content, "## Contributors")
	assert.Contains(t, result.Content, "@user5")
	assert.Contains(t, result.Content, "## New Contributors")
	compareURL := repo.HTMLURL(t.Context()) + "/compare/v1.1...v1.2.0"
	assert.Contains(t, result.Content, fmt.Sprintf("[v1.1...v1.2.0](%s)", compareURL))
}

func TestGenerateReleaseNotes_NoReleaseFallsBackToTags(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)

	mergedCommit := "90c1019714259b24fb81711d4416ac0f18667dfa"
	createMergedPullRequest(t, repo, mergedCommit, 5)

	_, err = db.GetEngine(t.Context()).
		Where("repo_id=?", repo.ID).
		Delete(new(repo_model.Release))
	require.NoError(t, err)

	result, err := GenerateReleaseNotes(t.Context(), repo, gitRepo, GenerateReleaseNotesOptions{
		TagName: "v1.2.0",
		Target:  "DefaultBranch",
	})
	require.NoError(t, err)
	assert.Equal(t, "v1.1", result.PreviousTag)
	assert.Contains(t, result.Content, "@user5")
}

func TestAutoPreviousReleaseTag_UsesPrevPublishedRelease(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	prev := insertTestRelease(ctx, t, repo, "auto-prev", timeutil.TimeStamp(100), releaseInsertOptions{})
	insertTestRelease(ctx, t, repo, "auto-draft", timeutil.TimeStamp(150), releaseInsertOptions{IsDraft: true})
	insertTestRelease(ctx, t, repo, "auto-pre", timeutil.TimeStamp(175), releaseInsertOptions{IsPrerelease: true})
	current := insertTestRelease(ctx, t, repo, "auto-current", timeutil.TimeStamp(200), releaseInsertOptions{})

	candidate, err := autoPreviousReleaseTag(ctx, repo, current.TagName)
	require.NoError(t, err)
	assert.Equal(t, prev.TagName, candidate)
}

func TestAutoPreviousReleaseTag_LatestReleaseFallback(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	latest := insertTestRelease(ctx, t, repo, "auto-latest", timeutil.TimeStampNow(), releaseInsertOptions{})

	candidate, err := autoPreviousReleaseTag(ctx, repo, "missing-tag")
	require.NoError(t, err)
	assert.Equal(t, latest.TagName, candidate)
}

func TestFindPreviousTagName(t *testing.T) {
	tags := []*git.Tag{
		{Name: "v2.0.0"},
		{Name: "v1.1.0"},
		{Name: "v1.0.0"},
	}

	prev, ok := findPreviousTagName(tags, "v1.1.0")
	require.True(t, ok)
	assert.Equal(t, "v1.0.0", prev)

	prev, ok = findPreviousTagName(tags, "v9.9.9")
	require.True(t, ok)
	assert.Equal(t, "v2.0.0", prev)

	_, ok = findPreviousTagName([]*git.Tag{}, "v1.0.0")
	assert.False(t, ok)
}

func createMergedPullRequest(t *testing.T, repo *repo_model.Repository, mergeCommit string, posterID int64) *issues_model.PullRequest {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: posterID})

	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Poster:   user,
		PosterID: user.ID,
		Title:    "Release notes test pull request",
		Content:  "content",
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: repo.ID,
		BaseRepoID: repo.ID,
		HeadBranch: repo.DefaultBranch,
		BaseBranch: repo.DefaultBranch,
		Status:     issues_model.PullRequestStatusMergeable,
		Flow:       issues_model.PullRequestFlowGithub,
	}

	require.NoError(t, issues_model.NewPullRequest(t.Context(), repo, issue, nil, nil, pr))

	pr.HasMerged = true
	pr.MergedCommitID = mergeCommit
	pr.MergedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(t.Context()).
		ID(pr.ID).
		Cols("has_merged", "merged_commit_id", "merged_unix").
		Update(pr)
	require.NoError(t, err)

	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.Issue.LoadAttributes(t.Context()))
	return pr
}

type releaseInsertOptions struct {
	IsDraft      bool
	IsPrerelease bool
	IsTag        bool
}

func insertTestRelease(ctx context.Context, t *testing.T, repo *repo_model.Repository, tag string, created timeutil.TimeStamp, opts releaseInsertOptions) *repo_model.Release {
	t.Helper()
	lower := strings.ToLower(tag)

	release := &repo_model.Release{
		RepoID:       repo.ID,
		PublisherID:  repo.OwnerID,
		TagName:      tag,
		LowerTagName: lower,
		Target:       repo.DefaultBranch,
		Title:        tag,
		Sha1:         fmt.Sprintf("%040d", int64(created)+time.Now().UnixNano()),
		IsDraft:      opts.IsDraft,
		IsPrerelease: opts.IsPrerelease,
		IsTag:        opts.IsTag,
		CreatedUnix:  created,
	}

	_, err := db.GetEngine(ctx).Insert(release)
	require.NoError(t, err)

	return release
}
