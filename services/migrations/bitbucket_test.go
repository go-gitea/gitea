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

// TestBitbucketDownloadMergedPR uses a recorded public PR to cover merged state, merge commit and comments.
func TestBitbucketDownloadMergedPR(t *testing.T) {
	liveMode := os.Getenv("BITBUCKET_RECORD") != ""

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestBitbucketDownloadMergedPR")
	mockServer := unittest.NewMockWebServer(t, "https://api.bitbucket.org", fixtureDir, liveMode, unittest.MockServerOptions{
		Routes: func(mux *http.ServeMux) {
			mux.HandleFunc("/2.0/repositories/atlassian/aui/issues", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusGone)
				_, _ = w.Write([]byte(`{"message":"CHANGE-3071 - Functionality has been deprecated"}`))
			})
		},
	})

	ctx := t.Context()
	downloader, err := NewBitbucketDownloader(ctx, mockServer.URL+"/2.0", "https://bitbucket.org", "atlassian", "aui", "", "", "")
	require.NoError(t, err)

	prs, isEnd, err := downloader.GetPullRequests(ctx, 1, 1)
	require.NoError(t, err)
	assert.False(t, isEnd)
	require.Len(t, prs, 1)
	pr := prs[0]
	assert.EqualValues(t, 1, pr.Number)
	assert.Equal(t, "closed", pr.State)
	assert.True(t, pr.Merged)
	assert.NotNil(t, pr.MergedTime)
	assert.Len(t, pr.MergeCommitSHA, 40)
	assert.Empty(t, pr.Head.SHA) // the source repository/commit of this old PR is gone (null in the API)
	assert.Len(t, pr.Base.SHA, 40)
	assert.Equal(t, "https://bitbucket.org/atlassian/aui/pull-requests/1/patch", pr.PatchURL)

	comments, isEnd, err := downloader.GetComments(ctx, pr)
	require.NoError(t, err)
	assert.True(t, isEnd)
	assert.Len(t, comments, 25)
	for _, c := range comments {
		assert.NotZero(t, c.Index)
		assert.NotEmpty(t, c.PosterName)
	}
}

// TestBitbucketAuthorization verifies credentials are sent to the API host only, as Bearer or Basic.
func TestBitbucketAuthorization(t *testing.T) {
	var got atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"slug":"r","mainbranch":{"name":"main"},"links":{"html":{"href":"https://bitbucket.org/w/r"},"clone":[{"name":"https","href":"https://bitbucket.org/w/r.git"}]}}`))
	}))
	defer server.Close()

	for _, c := range []struct{ user, password, token, want string }{
		{"", "", "", ""},
		{"u", "p", "", "Basic dTpw"},
		{"u", "p", "tok", "Bearer tok"},
	} {
		d, err := NewBitbucketDownloader(t.Context(), server.URL+"/2.0", "https://bitbucket.org", "w", "r", c.user, c.password, c.token)
		require.NoError(t, err)
		_, err = d.GetRepoInfo(t.Context())
		require.NoError(t, err)
		assert.Equal(t, c.want, got.Load(), "user=%q token=%q", c.user, c.token)
	}
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

func TestBitbucketIssueLabels(t *testing.T) {
	assert.Empty(t, bitbucketIssueLabels(bitbucketIssue{}))

	labels := bitbucketIssueLabels(bitbucketIssue{
		Kind:      "bug",
		Priority:  "major",
		Component: &bitbucketNamedValue{Name: "migrations"},
		Version:   &bitbucketNamedValue{Name: "1.0"},
	})
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	assert.Equal(t, []string{"kind/bug", "priority/major", "component/migrations", "version/1.0"}, names)
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

func TestBitbucketFormatCloneURL(t *testing.T) {
	d := &BitbucketDownloader{}
	const addr = "https://bitbucket.org/gitea/repo"

	got, err := d.FormatCloneURL(base.MigrateOptions{AuthUsername: "me@example.com", AuthPassword: "ATATT"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://x-bitbucket-api-token-auth:ATATT@bitbucket.org/gitea/repo", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthToken: "tok"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://x-token-auth:tok@bitbucket.org/gitea/repo", got)
}
