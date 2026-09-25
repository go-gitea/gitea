// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gitea.dev/models/unittest"
	base "gitea.dev/modules/migration"
	gitea_sdk "gitea.dev/sdk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForgejoDownloadRepo(t *testing.T) {
	token := os.Getenv("FORGEJO_TEST_TOKEN")

	_, callerFile, _, _ := runtime.Caller(0)
	mockServer := unittest.NewMockWebServer(t, "http://localhost:3000", filepath.Join(filepath.Dir(callerFile), "_mock_data/TestForgejoDownloadRepo"), token != "")

	ctx := t.Context()
	downloader, err := NewGiteaDownloader(ctx, mockServer.URL, "alice/test_repo", "", "", token)
	require.NoError(t, err)
	assert.True(t, downloader.pagination)

	repo, err := downloader.GetRepoInfo(ctx)
	require.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "alice",
		IsPrivate:     true,
		Description:   "Test repository for testing migration from forgejo to gitea",
		Website:       "https://forgejo.example/website",
		CloneURL:      "https://forgejo.example/alice/test_repo.git",
		OriginalURL:   "https://forgejo.example/alice/test_repo",
		DefaultBranch: "main",
	}, repo)

	topics, err := downloader.GetTopics(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"forgejo", "migration"}, topics)

	milestones, err := downloader.GetMilestones(ctx)
	require.NoError(t, err)
	require.Len(t, milestones, 2)
	assert.Equal(t, "v0", milestones[0].Title)
	assert.Equal(t, "closed", milestones[0].State)
	assert.Equal(t, "v1", milestones[1].Title)
	assert.Equal(t, "open", milestones[1].State)

	labels, err := downloader.GetLabels(ctx)
	require.NoError(t, err)
	assertLabelsEqual(t, []*base.Label{
		{Name: "kind/bug", Color: "ee0701", Description: "Something is broken", Exclusive: true},
		{Name: "kind/feature", Color: "0e8a16", Exclusive: true, Archived: true},
		{Name: "question", Color: "cc317c"},
	}, labels)

	releases, err := downloader.GetReleases(ctx)
	require.NoError(t, err)
	require.Len(t, releases, 2)
	assert.Equal(t, "v0.9.0", releases[0].TagName)
	assert.True(t, releases[0].Prerelease)
	assert.Equal(t, "v1.0.0", releases[1].TagName)
	var assets []string
	for _, asset := range releases[1].Assets {
		rc, err := asset.DownloadFunc()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		assets = append(assets, string(content))
	}
	assert.Equal(t, []string{"forgejo asset\n", "https://example.com/download.zip"}, assets)

	issues, isEnd, err := downloader.GetIssues(ctx, 1, 50)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, issues, 1)
	assert.Equal(t, "first issue", issues[0].Title)
	assert.Equal(t, "v1", issues[0].Milestone)
	assert.Equal(t, []*base.Reaction{{UserID: 1, UserName: "alice", Content: "heart"}, {UserID: 1, UserName: "alice", Content: "+1"}}, issues[0].Reactions)

	comments, _, err := downloader.GetComments(ctx, issues[0])
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "a comment", comments[0].Content)
	assert.Equal(t, []*base.Reaction{{UserID: 1, UserName: "alice", Content: "laugh"}}, comments[0].Reactions)

	prs, _, err := downloader.GetPullRequests(ctx, 1, 50)
	require.NoError(t, err)
	require.Len(t, prs, 3)
	agit, open, merged := prs[0], prs[1], prs[2]
	assert.Equal(t, "refs/pull/4/head", agit.Head.Ref)
	assert.Equal(t, "feature-open", open.Head.Ref)
	assert.True(t, merged.Merged)
	assert.Equal(t, "refs/pull/2/head", merged.Head.Ref)
	assert.Equal(t, "23181c5c8c25740ab7f080faf27cbcae7fd13519", merged.Base.SHA)
	assert.Equal(t, "588ba39bdea707266f2df950dcee1bb5a1764fed", merged.MergeCommitSHA)

	comments, _, err = downloader.GetComments(ctx, agit)
	require.NoError(t, err)
	assert.Empty(t, comments)

	reviews, err := downloader.GetReviews(ctx, merged)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.Equal(t, base.ReviewStateApproved, reviews[0].State)

	reviews, err = downloader.GetReviews(ctx, open)
	require.NoError(t, err)
	require.Len(t, reviews, 2)
	assert.Equal(t, base.ReviewStateChangesRequested, reviews[0].State)
	assert.True(t, reviews[0].Dismissed)
	require.Len(t, reviews[0].Comments, 1)
	assert.Equal(t, "fix this", reviews[0].Comments[0].Content)
	assert.Equal(t, base.ReviewStateCommented, reviews[1].State)
	assert.False(t, reviews[1].Dismissed)
}

func TestGiteaDownloaderPreReleaseServerVersion(t *testing.T) {
	for rawVersion, pagination := range map[string]bool{
		"1.11.0":              false,
		"1.12.0-rc1":          true,
		"11.0.0+gitea-1.11.0": false,
		"development":         true,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/version" {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintf(w, `{"version":%q}`, rawVersion)
		}))
		downloader, err := NewGiteaDownloader(t.Context(), server.URL, "owner/repo", "", "", "")
		server.Close()
		require.NoError(t, err)
		assert.Equal(t, pagination, downloader.pagination, rawVersion)
	}
}

func TestGiteaDownloaderDisabledUnits(t *testing.T) {
	for _, unitsEnabled := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/version":
				_, _ = w.Write([]byte(`{"version":"1.27.0"}`))
			case "/api/v1/repos/owner/repo":
				_, _ = fmt.Fprintf(w, `{"has_issues":%[1]t,"has_pull_requests":%[1]t,"has_releases":%[1]t}`, unitsEnabled)
			default:
				http.NotFound(w, r)
			}
		}))

		ctx := t.Context()
		downloader, err := NewGiteaDownloader(ctx, server.URL, "owner/repo", "", "", "")
		require.NoError(t, err)
		_, issuesEnd, issuesErr := downloader.GetIssues(ctx, 1, 10)
		_, prsEnd, prsErr := downloader.GetPullRequests(ctx, 1, 10)
		_, releasesErr := downloader.GetReleases(ctx)
		server.Close()

		assert.Equal(t, unitsEnabled, issuesErr != nil)
		assert.Equal(t, unitsEnabled, prsErr != nil)
		assert.Equal(t, unitsEnabled, releasesErr != nil)
		assert.Equal(t, !unitsEnabled, issuesEnd)
		assert.Equal(t, !unitsEnabled, prsEnd)
	}
}

func TestGiteaDownloaderIsHostedAttachment(t *testing.T) {
	downloader := &GiteaDownloader{baseURL: "http://127.0.0.1:3000"}
	release := &gitea_sdk.Release{TagName: "v1.0.0", HTMLURL: "https://forgejo.example/owner/repo/releases/tag/v1.0.0"}
	for downloadURL, hosted := range map[string]bool{
		"https://forgejo.example/owner/repo/releases/download/v1.0.0/file.zip": true,
		"http://127.0.0.1:3000/attachments/abc":                                true,
		"https://github.com/owner/repo/releases/download/v1.0.0/file.zip":      false,
		"https://forgejo.example/elsewhere/file.zip":                           false,
	} {
		assert.Equal(t, hosted, downloader.isHostedAttachment(release, &gitea_sdk.Attachment{UUID: "abc", Name: "file.zip", DownloadURL: downloadURL}), downloadURL)
	}
	assert.False(t, downloader.isHostedAttachment(release, &gitea_sdk.Attachment{Name: "file.zip", DownloadURL: "http://127.0.0.1:3000/attachments/"}))
}
