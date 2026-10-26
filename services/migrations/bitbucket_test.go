// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitbucketDownloadRepo(t *testing.T) {
	token := os.Getenv("BITBUCKET_READ_TOKEN")
	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestBitbucketDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, "https://api.bitbucket.org", fixtureDir, token != "")
	ctx := t.Context()

	downloader := NewBitbucketDownloader(mockServer.URL+"/2.0", "gitea", "test_repo", "", "", token)
	repo, err := downloader.GetRepoInfo(ctx)
	require.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "gitea",
		CloneURL:      "https://bitbucket.org/gitea/test_repo.git",
		OriginalURL:   "https://bitbucket.org/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	prs, isEnd, err := downloader.GetPullRequests(ctx, 1, 10)
	require.NoError(t, err)
	assert.True(t, isEnd)
	assertPullRequestsEqual(t, []*base.PullRequest{{
		Number:     1,
		PosterID:   5793964208530042003,
		PosterName: "Lunny Xiao",
		Title:      "Update LICENSE",
		Content:    "do not merge this PR",
		State:      "open",
		Created:    time.Date(2026, 10, 7, 18, 39, 55, 551762000, time.UTC),
		Updated:    time.Date(2026, 10, 7, 18, 40, 40, 122213000, time.UTC),
		Head: base.PullRequestBranch{
			CloneURL:  "https://bitbucket.org/gitea/test_repo.git",
			Ref:       "feat/test",
			SHA:       "9f733b96b98a4175276edf6a2e1231489c3bdd23",
			OwnerName: "gitea",
			RepoName:  "test_repo",
		},
		Base: base.PullRequestBranch{
			Ref:       "master",
			SHA:       "c59c9b451acca9d106cc19d61d87afe3fbbb8b83",
			OwnerName: "gitea",
			RepoName:  "test_repo",
		},
		ForeignIndex: 1,
		EnsuredSafe:  true,
	}}, prs)

	downloader = NewBitbucketDownloader(mockServer.URL+"/2.0", "atlassian", "aui", "", "", token)
	prs, isEnd, err = downloader.GetPullRequests(ctx, 1, 1)
	require.NoError(t, err)
	assert.False(t, isEnd)
	require.Len(t, prs, 1)
	assert.Equal(t, "closed", prs[0].State)
	assert.True(t, prs[0].Merged)
	assert.NotNil(t, prs[0].MergedTime)
	assert.Len(t, prs[0].MergeCommitSHA, 40)
	assert.Empty(t, prs[0].Head.SHA)
	assert.Len(t, prs[0].Base.SHA, 40)

	comments, isEnd, err := downloader.GetComments(ctx, prs[0])
	require.NoError(t, err)
	assert.True(t, isEnd)
	assert.Len(t, comments, 25)
}

func TestBitbucketReviews(t *testing.T) {
	var commentRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2.0/repositories/w/r/pullrequests":
			_, _ = w.Write([]byte(`{"values":[{"id":3,"state":"OPEN","updated_on":"2024-01-03T00:00:00Z","source":{"commit":{"hash":"` + strings.Repeat("a", 40) + `"},"repository":{"full_name":"w/r"}},"participants":[
				{"user":{"nickname":"alice"},"approved":true,"participated_on":"2024-01-02T00:00:00Z"},
				{"user":{"nickname":"bob"},"state":"changes_requested"},
				{"user":{"nickname":"carol"}}]}]}`))
		case "/2.0/repositories/w/r/pullrequests/3/comments":
			commentRequests.Add(1)
			_, _ = w.Write([]byte(`{"values":[
				{"id":1,"content":{"raw":"general"},"user":{"nickname":"alice"}},
				{"id":2,"content":{"raw":"inline"},"user":{"nickname":"bob"},"inline":{"path":"a.txt","to":5},"resolution":{}},
				{"id":3,"content":{"raw":"old side"},"user":{"nickname":"bob"},"inline":{"path":"a.txt","from":7},"parent":{"id":2}},
				{"id":4,"content":{"raw":"gone"},"user":{"nickname":"bob"},"deleted":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	d := NewBitbucketDownloader(server.URL+"/2.0", "w", "r", "", "", "")
	prs, _, err := d.GetPullRequests(t.Context(), 1, 10)
	require.NoError(t, err)
	require.Len(t, prs, 1)

	comments, _, err := d.GetComments(t.Context(), prs[0])
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "general", comments[0].Content)

	reviews, err := d.GetReviews(t.Context(), prs[0])
	require.NoError(t, err)
	assert.EqualValues(t, 1, commentRequests.Load())
	var inline, states []string
	for _, r := range reviews {
		if len(r.Comments) > 0 {
			rc := r.Comments[0]
			inline = append(inline, fmt.Sprintf("%s:%d:%v", rc.TreePath, rc.Line, rc.Resolved))
			assert.Equal(t, strings.Repeat("a", 40), rc.CommitID)
		} else {
			states = append(states, r.ReviewerName+":"+r.State)
		}
	}
	assert.ElementsMatch(t, []string{"a.txt:5:true", "a.txt:-7:false"}, inline)
	assert.ElementsMatch(t, []string{"alice:APPROVED", "bob:CHANGES_REQUESTED"}, states)
}

func TestBitbucketCloneURL(t *testing.T) {
	d := &BitbucketDownloader{}
	const addr = "https://bitbucket.org/gitea/repo"

	got, err := d.FormatCloneURL(base.MigrateOptions{AuthUsername: "me@example.com", AuthPassword: "ATATT"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://x-bitbucket-api-token-auth:ATATT@bitbucket.org/gitea/repo", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthToken: "tok"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://x-token-auth:tok@bitbucket.org/gitea/repo", got)

	_, err = (&BitbucketDownloaderFactory{}).New(t.Context(), base.MigrateOptions{CloneAddr: "https://git.example.com/w/r.git", AuthUsername: "u", AuthPassword: "p"})
	assert.ErrorContains(t, err, "host must be bitbucket.org")
}
