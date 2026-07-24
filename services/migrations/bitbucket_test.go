// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitbucketDownloadRepo(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/2.0/repositories/gitea/test-repo":
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
					"clone":[{"name":"ssh","href":"git@bitbucket.org:gitea/test-repo.git"},{"name":"https","href":"%[1]s/gitea/test-repo.git"}]
				}
			}`, serverURL)
		case "/2.0/repositories/gitea/test-repo/issues":
			writeBitbucketIssuesPage(w, r, serverURL)
		case "/2.0/repositories/gitea/test-repo/issues/2/comments":
			_, _ = fmt.Fprint(w, `{"values":[{
				"id":11,
				"content":{"raw":"This is a Bitbucket issue comment"},
				"user":{"account_id":"user-1","nickname":"alice"},
				"created_on":"2020-01-03T12:00:00Z",
				"updated_on":"2020-01-03T12:10:00Z"
			}]}`)
		case "/2.0/repositories/gitea/test-repo/pullrequests":
			_, _ = fmt.Fprintf(w, `{"values":[{
				"id":1,
				"title":"Add Bitbucket migration",
				"description":"Implements migration support",
				"state":"MERGED",
				"author":{"account_id":"user-2","nickname":"bob"},
				"source":{
					"branch":{"name":"feature/bitbucket"},
					"commit":{"hash":"1111111111111111111111111111111111111111"},
					"repository":{"slug":"test-repo","full_name":"bob/test-repo","links":{"clone":[{"name":"https","href":"%[1]s/bob/test-repo.git"}]}}
				},
				"destination":{
					"branch":{"name":"main"},
					"commit":{"hash":"2222222222222222222222222222222222222222"},
					"repository":{"slug":"test-repo","full_name":"gitea/test-repo","links":{"clone":[{"name":"https","href":"%[1]s/gitea/test-repo.git"}]}}
				},
				"merge_commit":{"hash":"3333333333333333333333333333333333333333"},
				"links":{"patch":{"href":"%[1]s/gitea/test-repo/pull-requests/1.patch"}},
				"created_on":"2020-01-04T12:00:00Z",
				"updated_on":"2020-01-05T12:00:00Z"
			}]}`, serverURL)
		case "/2.0/repositories/gitea/test-repo/pullrequests/1/comments":
			_, _ = fmt.Fprint(w, `{"values":[{
				"id":21,
				"content":{"raw":"This is a Bitbucket pull request comment"},
				"user":{"account_id":"user-1","nickname":"alice"},
				"created_on":"2020-01-04T13:00:00Z",
				"updated_on":"2020-01-04T13:00:00Z"
			}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	ctx := t.Context()
	downloader, err := NewBitbucketDownloader(ctx, server.URL+"/2.0", server.URL, "gitea", "test-repo", "", "", "")
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test-repo",
		Owner:         "gitea",
		Description:   "Test repository for testing migration from bitbucket to gitea",
		Website:       "https://gitea.com/test-repo",
		CloneURL:      server.URL + "/gitea/test-repo.git",
		OriginalURL:   server.URL + "/gitea/test-repo",
		DefaultBranch: "main",
	}, repo)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	sort.Slice(labels, func(i, j int) bool { return labels[i].Name < labels[j].Name })
	assertLabelsEqual(t, []*base.Label{
		{Name: "component/migrations", Color: bitbucketLabelColor("component/migrations")},
		{Name: "kind/bug", Color: bitbucketLabelColor("kind/bug")},
		{Name: "kind/enhancement", Color: bitbucketLabelColor("kind/enhancement")},
		{Name: "priority/major", Color: bitbucketLabelColor("priority/major")},
		{Name: "priority/minor", Color: bitbucketLabelColor("priority/minor")},
		{Name: "version/1.0", Color: bitbucketLabelColor("version/1.0")},
	}, labels)

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	sort.Slice(milestones, func(i, j int) bool { return milestones[i].Title < milestones[j].Title })
	assertMilestonesEqual(t, []*base.Milestone{
		{Title: "v1", Created: time.Unix(0, 0), State: "open"},
	}, milestones)

	issues, isEnd, err := downloader.GetIssues(ctx, 1, 1)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{{
		Number:     1,
		PosterID:   bitbucketUserID(bitbucketUser{AccountID: "user-1"}),
		PosterName: "alice",
		Title:      "Open issue",
		Content:    "Issue body",
		Milestone:  "v1",
		State:      "open",
		Created:    time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC),
		Updated:    time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC),
		Labels: []*base.Label{
			{Name: "kind/bug", Color: bitbucketLabelColor("kind/bug")},
			{Name: "priority/major", Color: bitbucketLabelColor("priority/major")},
			{Name: "component/migrations", Color: bitbucketLabelColor("component/migrations")},
			{Name: "version/1.0", Color: bitbucketLabelColor("version/1.0")},
		},
		Assignees: []string{"bob"},
	}}, issues)

	issues, isEnd, err = downloader.GetIssues(ctx, 2, 1)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{{
		Number:     2,
		PosterID:   bitbucketUserID(bitbucketUser{AccountID: "user-2"}),
		PosterName: "bob",
		Title:      "Closed issue",
		Content:    "Closed body",
		State:      "closed",
		Created:    time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC),
		Updated:    time.Date(2020, 1, 3, 12, 0, 0, 0, time.UTC),
		Closed:     new(time.Date(2020, 1, 3, 12, 0, 0, 0, time.UTC)),
		Labels: []*base.Label{
			{Name: "kind/enhancement", Color: bitbucketLabelColor("kind/enhancement")},
			{Name: "priority/minor", Color: bitbucketLabelColor("priority/minor")},
		},
	}}, issues)

	comments, _, err := downloader.GetComments(ctx, &base.Issue{Number: 2, ForeignIndex: 2})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{{
		IssueIndex: 2,
		PosterID:   bitbucketUserID(bitbucketUser{AccountID: "user-1"}),
		PosterName: "alice",
		Created:    time.Date(2020, 1, 3, 12, 0, 0, 0, time.UTC),
		Updated:    time.Date(2020, 1, 3, 12, 10, 0, 0, time.UTC),
		Content:    "This is a Bitbucket issue comment",
	}}, comments)

	prs, isEnd, err := downloader.GetPullRequests(ctx, 1, 10)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assertPullRequestsEqual(t, []*base.PullRequest{{
		Number:         3,
		PosterID:       bitbucketUserID(bitbucketUser{AccountID: "user-2"}),
		PosterName:     "bob",
		Title:          "Add Bitbucket migration",
		Content:        "Implements migration support",
		State:          "closed",
		Created:        time.Date(2020, 1, 4, 12, 0, 0, 0, time.UTC),
		Updated:        time.Date(2020, 1, 5, 12, 0, 0, 0, time.UTC),
		Closed:         new(time.Date(2020, 1, 5, 12, 0, 0, 0, time.UTC)),
		PatchURL:       server.URL + "/gitea/test-repo/pull-requests/1.patch",
		Merged:         true,
		MergedTime:     new(time.Date(2020, 1, 5, 12, 0, 0, 0, time.UTC)),
		MergeCommitSHA: "3333333333333333333333333333333333333333",
		Head: base.PullRequestBranch{
			CloneURL:  server.URL + "/bob/test-repo.git",
			Ref:       "feature/bitbucket",
			SHA:       "1111111111111111111111111111111111111111",
			OwnerName: "bob",
			RepoName:  "test-repo",
		},
		Base: base.PullRequestBranch{
			Ref:       "main",
			SHA:       "2222222222222222222222222222222222222222",
			OwnerName: "gitea",
			RepoName:  "test-repo",
		},
		ForeignIndex: 1,
		EnsuredSafe:  true,
	}}, prs)

	comments, _, err = downloader.GetComments(ctx, prs[0])
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{{
		IssueIndex: 3,
		PosterID:   bitbucketUserID(bitbucketUser{AccountID: "user-1"}),
		PosterName: "alice",
		Created:    time.Date(2020, 1, 4, 13, 0, 0, 0, time.UTC),
		Updated:    time.Date(2020, 1, 4, 13, 0, 0, 0, time.UTC),
		Content:    "This is a Bitbucket pull request comment",
	}}, comments)
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
