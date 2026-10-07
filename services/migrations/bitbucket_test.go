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
	"strconv"
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
	liveMode := token != ""

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestBitbucketDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, "https://api.bitbucket.org", fixtureDir, liveMode, unittest.MockServerOptions{
		Routes: func(mux *http.ServeMux) {
			// bitbucket.org has retired the issue tracker API (CHANGE-3071) and answers 410 Gone
			mux.HandleFunc("/2.0/repositories/gitea/test_repo/issues", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusGone)
				_, _ = w.Write([]byte(`{"message":"CHANGE-3071 - Functionality has been deprecated"}`))
			})
		},
	})

	ctx := t.Context()
	downloader, err := NewBitbucketDownloader(ctx, mockServer.URL+"/2.0", "https://bitbucket.org", "gitea", "test_repo", "", "", token)
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "gitea",
		CloneURL:      "https://bitbucket.org/gitea/test_repo.git",
		OriginalURL:   "https://bitbucket.org/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	assert.Empty(t, labels)

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	assert.Empty(t, milestones)

	_, _, err = downloader.GetIssues(ctx, 1, 10)
	assert.ErrorIs(t, err, base.ErrNotSupported{Entity: "Issues"})

	prs, isEnd, err := downloader.GetPullRequests(ctx, 1, 10)
	assert.NoError(t, err)
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
		PatchURL:   "https://bitbucket.org/gitea/test_repo/pull-requests/1/patch",
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

	comments, _, err := downloader.GetComments(ctx, prs[0])
	assert.NoError(t, err)
	assert.Empty(t, comments)
}

// TestBitbucketRateLimitRetry verifies that doAPI transparently waits and retries when the
// Bitbucket API answers with HTTP 429 Too Many Requests before eventually succeeding.
func TestBitbucketRateLimitRetry(t *testing.T) {
	const failuresBeforeSuccess = 2

	var serverURL string
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/gitea/test-repo" {
			http.NotFound(w, r)
			return
		}

		// The first few attempts are rate limited; Retry-After: 0 keeps the test fast.
		if attempts.Add(1) <= failuresBeforeSuccess {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":{"message":"Rate limit exceeded"}}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"name":"Test Repo",
			"slug":"test-repo",
			"full_name":"gitea/test-repo",
			"description":"Test repository for testing migration from bitbucket to gitea",
			"is_private":false,
			"website":"https://gitea.com/test-repo",
			"mainbranch":{"name":"main"},
			"links":{
				"html":{"href":"%[1]s/gitea/test-repo"},
				"clone":[{"name":"https","href":"%[1]s/gitea/test-repo.git"}]
			}
		}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	ctx := t.Context()
	downloader, err := NewBitbucketDownloader(ctx, server.URL+"/2.0", server.URL, "gitea", "test-repo", "", "", "")
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test-repo", repo.Name)
	assert.Equal(t, int32(failuresBeforeSuccess+1), attempts.Load())
}

func writeBitbucketIssuesPage(w http.ResponseWriter, r *http.Request, serverURL string) {
	if r.URL.Query().Get("pagelen") == "1" && r.URL.Query().Get("page") == "1" {
		_, _ = fmt.Fprintf(w, `{"next":"%s/2.0/repositories/gitea/test-repo/issues?page=2","values":[%s]}`, serverURL, bitbucketIssueJSON(1))
		return
	}
	if r.URL.Query().Get("pagelen") == "1" && r.URL.Query().Get("page") == "2" {
		_, _ = fmt.Fprintf(w, `{"values":[%s]}`, bitbucketIssueJSON(2))
		return
	}
	_, _ = fmt.Fprintf(w, `{"values":[%s,%s]}`, bitbucketIssueJSON(1), bitbucketIssueJSON(2))
}

func bitbucketIssueJSON(id int) string {
	if id == 1 {
		return `{
			"id":1,
			"title":"Open issue",
			"content":{"raw":"Issue body"},
			"state":"open",
			"kind":"bug",
			"priority":"major",
			"reporter":{"account_id":"user-1","nickname":"alice"},
			"assignee":{"account_id":"user-2","nickname":"bob"},
			"component":{"name":"migrations"},
			"version":{"name":"1.0"},
			"milestone":{"name":"v1"},
			"created_on":"2020-01-01T12:00:00Z",
			"updated_on":"2020-01-02T12:00:00Z"
		}`
	}
	return `{
		"id":2,
		"title":"Closed issue",
		"content":{"raw":"Closed body"},
		"state":"resolved",
		"kind":"enhancement",
		"priority":"minor",
		"reporter":{"account_id":"user-2","nickname":"bob"},
		"created_on":"2020-01-02T12:00:00Z",
		"updated_on":"2020-01-03T12:00:00Z"
	}`
}

func TestBitbucketRetryWait(t *testing.T) {
	// retryWaitFor builds a bodyless response carrying the given headers and returns the
	// wait that bitbucketRetryWait computes for it.
	retryWaitFor := func(headers map[string]string) time.Duration {
		resp := &http.Response{Header: http.Header{}}
		for k, v := range headers {
			resp.Header.Set(k, v)
		}
		return bitbucketRetryWait(resp, 0)
	}

	// Retry-After in seconds is honored exactly.
	assert.Equal(t, 5*time.Second, retryWaitFor(map[string]string{"Retry-After": "5"}))

	// Bitbucket's X-RateLimit-Reset is a seconds-remaining delta, not an epoch.
	assert.Equal(t, 120*time.Second, retryWaitFor(map[string]string{"X-RateLimit-Reset": "120"}))

	// A delta larger than the cap is clamped.
	assert.Equal(t, bitbucketMaxRetryAfter, retryWaitFor(map[string]string{"X-RateLimit-Reset": "999999"}))

	// A value large enough to be a Unix epoch is treated as an absolute timestamp.
	epoch := strconv.FormatInt(time.Now().Add(5*time.Minute).Unix(), 10)
	got := retryWaitFor(map[string]string{"X-RateLimit-Reset": epoch})
	assert.InDelta(t, (5 * time.Minute).Seconds(), got.Seconds(), 5)

	// An RFC3339 timestamp a few minutes in the future is honored as an absolute time.
	rfc := time.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339)
	assert.InDelta(t, (3 * time.Minute).Seconds(), retryWaitFor(map[string]string{"X-RateLimit-Reset": rfc}).Seconds(), 5)

	// With no rate-limit headers it falls back to a positive exponential backoff.
	assert.Positive(t, retryWaitFor(nil))
}
