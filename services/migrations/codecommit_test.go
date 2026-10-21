// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/require"
)

func TestCodeCommitDownloadRepo(t *testing.T) {
	accessKeyID, secretAccessKey := os.Getenv("CODECOMMIT_ACCESS_KEY_ID"), os.Getenv("CODECOMMIT_SECRET_ACCESS_KEY")
	mockServer := unittest.NewMockWebServer(t, "https://codecommit.us-east-1.amazonaws.com", "_mock_data/TestCodeCommitDownloadRepo", accessKeyID != "", unittest.MockServerOptions{
		FixtureName: func(r *http.Request, body []byte) string {
			return r.Header.Get("X-Amz-Target") + "_" + url.QueryEscape(string(body))
		},
	})
	proxyURL, _ := url.Parse(mockServer.URL)

	ctx := t.Context()
	downloader := NewCodeCommitDownloader(ctx, "test_repo", "https://git-codecommit.us-east-1.amazonaws.com", accessKeyID, secretAccessKey, "us-east-1")
	downloader.endpoint = "http://codecommit.us-east-1.amazonaws.com"
	downloader.client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	repo, err := downloader.GetRepoInfo(ctx)
	require.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "123456789012",
		IsPrivate:     true,
		Description:   "Test repository for Gitea migrations",
		CloneURL:      "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/test_repo",
		DefaultBranch: "main",
	}, repo)

	prs, _, err := downloader.GetPullRequests(ctx, 1, 1)
	require.NoError(t, err)
	mergedTime := time.Date(2026, time.September, 16, 16, 5, 13, 0, time.UTC)
	assertPullRequestsEqual(t, []*base.PullRequest{{
		Number:         4,
		Title:          "Test pull request",
		PosterName:     "gitea-codecommit",
		Content:        "Test pull request description",
		State:          "closed",
		Created:        time.Date(2026, time.September, 16, 16, 5, 8, 0, time.UTC),
		Updated:        mergedTime,
		Closed:         &mergedTime,
		Merged:         true,
		MergedTime:     &mergedTime,
		MergeCommitSHA: "43bddef3095d527c6d95d89d0ce992e2e79eeb76",
		Head:           base.PullRequestBranch{Ref: "feature", SHA: "43bddef3095d527c6d95d89d0ce992e2e79eeb76", RepoName: "test_repo"},
		Base:           base.PullRequestBranch{Ref: "main", SHA: "73bb36ac7a26a4d595f5b0bb892aeaa85a078ee7", RepoName: "test_repo"},
	}}, prs)

	comments, _, err := downloader.GetComments(ctx, prs[0])
	require.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{{
		IssueIndex: 4,
		PosterName: "gitea-codecommit",
		Content:    "Test comment",
		Created:    time.Date(2026, time.September, 16, 16, 5, 10, 0, time.UTC),
		Updated:    time.Date(2026, time.September, 16, 16, 5, 10, 0, time.UTC),
	}}, comments)
}
