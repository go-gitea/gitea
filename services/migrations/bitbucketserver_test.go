// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"cmp"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBitbucketServerURL(t *testing.T) {
	cases := []struct {
		in                 string
		baseURL, key, slug string
		wantErr            bool
	}{
		{in: "https://bb.example.com/scm/PROJ/repo.git", baseURL: "https://bb.example.com", key: "PROJ", slug: "repo"},
		{in: "https://user:pass@bb.example.com:7990/scm/proj/repo", baseURL: "https://bb.example.com:7990", key: "proj", slug: "repo"},
		{in: "https://bb.example.com/stash/scm/~jdoe/repo.git", baseURL: "https://bb.example.com/stash", key: "~jdoe", slug: "repo"},
		{in: "https://bb.example.com/projects/PROJ/repos/repo/browse", baseURL: "https://bb.example.com", key: "PROJ", slug: "repo"},
		{in: "https://bb.example.com/ctx/a/users/jdoe/repos/repo/pull-requests/1", baseURL: "https://bb.example.com/ctx/a", key: "~jdoe", slug: "repo"},
		{in: "https://bb.example.com/projects/PROJ", wantErr: true},
		{in: "https://bb.example.com/scm/PROJ", wantErr: true},
		{in: "ssh://git@bb.example.com/PROJ/repo.git", wantErr: true},
		{in: "/local/path/scm/PROJ/repo.git", wantErr: true},
	}
	for _, c := range cases {
		baseURL, key, slug, err := parseBitbucketServerURL(c.in)
		if c.wantErr {
			assert.Error(t, err, c.in)
			continue
		}
		require.NoError(t, err, c.in)
		assert.Equal(t, c.baseURL, baseURL, c.in)
		assert.Equal(t, c.key, key, c.in)
		assert.Equal(t, c.slug, slug, c.in)
	}
}

func TestBitbucketServerFormatCloneURL(t *testing.T) {
	d := NewBitbucketServerDownloader(t.Context(), "https://bb.example.com", "", "", "", "PROJ", "repo")
	const addr = "https://bb.example.com/scm/PROJ/repo.git"

	got, err := d.FormatCloneURL(base.MigrateOptions{}, addr)
	require.NoError(t, err)
	assert.Equal(t, addr, got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthUsername: "u", AuthPassword: "p"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://u:p@bb.example.com/scm/PROJ/repo.git", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthToken: "tok"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://token:tok@bb.example.com/scm/PROJ/repo.git", got)

	// git needs the owner of the token as user name, which is learned from the API responses
	d.tokenUser = "alice"
	got, err = d.FormatCloneURL(base.MigrateOptions{AuthToken: "tok"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://alice:tok@bb.example.com/scm/PROJ/repo.git", got)
	got, err = d.FormatCloneURL(base.MigrateOptions{AuthToken: "tok", AuthUsername: "bob"}, addr)
	require.NoError(t, err)
	assert.Equal(t, "https://bob:tok@bb.example.com/scm/PROJ/repo.git", got)

	assert.Equal(t, "j.doe", bitbucketServerGitUser("j.doe"))
	// project and repository access tokens report a technical user
	assert.Equal(t, "x-token-auth", bitbucketServerGitUser("access-token-user%2F1%2F2"))
	assert.Equal(t, "x-token-auth", bitbucketServerGitUser("access-token-user/2/2"))
	assert.Empty(t, bitbucketServerGitUser(""))
}

const bbsRepoJSON = `{"slug":"repo","name":"Repo","description":"a test repo","public":false,"project":{"key":"PROJ"}}`

func TestBitbucketServerRepoInfo(t *testing.T) {
	var requests []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("X-AUSERNAME", "alice")
		switch r.URL.Path {
		case "/rest/api/1.0/projects/PROJ/repos/repo", "/rest/api/1.0/projects/~jdoe/repos/repo", "/rest/api/1.0/projects/DENIED/repos/repo":
			_, _ = w.Write([]byte(bbsRepoJSON))
		case "/rest/api/1.0/projects/~jdoe/repos/repo/default-branch":
			_, _ = w.Write([]byte(`{"displayId":"main"}`))
		case "/rest/api/1.0/projects/DENIED/repos/repo/default-branch":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			http.NotFound(w, r) // an empty repository has no default branch
		}
	}))
	defer srv.Close()

	d := NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "secret", "PROJ", "repo")
	repo, err := d.GetRepoInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, &base.Repository{
		Name:        "repo",
		Owner:       "PROJ",
		IsPrivate:   true,
		Description: "a test repo",
		CloneURL:    srv.URL + "/scm/PROJ/repo.git",
		OriginalURL: srv.URL + "/projects/PROJ/repos/repo",
	}, repo)
	assert.Equal(t, "Bearer secret", requests[0].Header.Get("Authorization"))
	assert.Equal(t, "alice", d.tokenUser)
	_, err = d.GetReleases(t.Context())
	assert.True(t, base.IsErrNotSupported(err), "tags are synced from git")

	requests = nil
	repo, err = NewBitbucketServerDownloader(t.Context(), srv.URL, "u", "p", "", "~jdoe", "repo").GetRepoInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "main", repo.DefaultBranch)
	assert.Equal(t, srv.URL+"/users/jdoe/repos/repo", repo.OriginalURL)
	user, pass, ok := requests[0].BasicAuth()
	assert.True(t, ok)
	assert.Equal(t, "u", user)
	assert.Equal(t, "p", pass)

	_, err = NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "", "DENIED", "repo").GetRepoInfo(t.Context())
	assert.True(t, IsAuthenticationError(err))
}

const bbsPullRequestsPage1JSON = `{"isLastPage":false,"nextPageStart":1,"values":[
{"id":1,"title":"merged","description":"d1","state":"MERGED","createdDate":1700000000000,"updatedDate":1700000300000,"closedDate":1700000200000,
 "author":{"user":{"id":7,"name":"alice","emailAddress":"alice@example.com"}},
 "fromRef":{"displayId":"feature","latestCommit":"HEAD_SHA","repository":{"slug":"repo","project":{"key":"PROJ"}}},
 "toRef":{"displayId":"main","latestCommit":"BASE_SHA","repository":{"slug":"repo","project":{"key":"PROJ"}}}}
]}`

const bbsPullRequestsPage2JSON = `{"isLastPage":true,"values":[
{"id":2,"title":"fork pr","state":"OPEN","draft":true,"createdDate":1700001000000,"updatedDate":1700001000000,
 "author":{"user":{"id":8,"slug":"bob"}},
 "fromRef":{"displayId":"patch","latestCommit":"HEAD_SHA","repository":{"slug":"repo-fork","project":{"key":"~BOB"}}},
 "toRef":{"displayId":"main","latestCommit":"BASE_SHA","repository":{"slug":"repo","project":{"key":"PROJ"}}}},
{"id":3,"title":"declined","state":"DECLINED","createdDate":1700002000000,"updatedDate":1700002500000,
 "author":{"user":{"id":7,"name":"alice"}},
 "fromRef":{"displayId":"x","latestCommit":"not-a-sha","repository":{"slug":"repo","project":{"key":"PROJ"}}},
 "toRef":{"displayId":"main","latestCommit":"BASE_SHA","repository":{"slug":"repo","project":{"key":"PROJ"}}}}
]}`

func TestBitbucketServerPullRequests(t *testing.T) {
	headSHA, baseSHA, mergeSHA, forkPoint := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40), strings.Repeat("4", 40)
	const api = "/rest/api/1.0/projects/PROJ/repos/repo"
	shas := strings.NewReplacer("HEAD_SHA", headSHA, "BASE_SHA", baseSHA)
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case api + "/pull-requests/1/commits":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":"c2","parents":[{"id":"c1"}]},{"id":"c1","parents":[{"id":"` + forkPoint + `"}]}]}`))
		case api + "/pull-requests/1/activities":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"action":"MERGED","commit":{"id":"` + mergeSHA + `"}}]}`))
		case api + "/pull-requests":
			queries = append(queries, r.URL.RawQuery)
			body := bbsPullRequestsPage1JSON
			if r.URL.Query().Get("start") != "0" {
				body = bbsPullRequestsPage2JSON
			}
			_, _ = w.Write([]byte(shas.Replace(body)))
		default:
			assert.Failf(t, "unexpected request", "%s", r.URL.Path)
		}
	}))
	defer srv.Close()

	d := NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "", "PROJ", "repo")
	prs, isEnd, err := d.GetPullRequests(t.Context(), 1, 500)
	require.NoError(t, err)
	assert.False(t, isEnd)
	require.Len(t, prs, 1)
	page2, isEnd, err := d.GetPullRequests(t.Context(), 2, 500)
	require.NoError(t, err)
	assert.True(t, isEnd)
	prs = append(prs, page2...)
	// the page size is capped, and the server answering with fewer values must not skip any
	assert.Equal(t, []string{"limit=100&order=oldest&start=0&state=ALL", "limit=100&order=oldest&start=1&state=ALL"}, queries)
	require.Len(t, prs, 3)

	merged := prs[0]
	assert.EqualValues(t, 1, merged.Number)
	assert.EqualValues(t, 1, merged.GetForeignIndex())
	assert.Equal(t, "closed", merged.State)
	assert.True(t, merged.Merged)
	assert.Equal(t, mergeSHA, merged.MergeCommitSHA)
	assert.Equal(t, time.UnixMilli(1700000200000), *merged.MergedTime)
	assert.Equal(t, time.UnixMilli(1700000200000), *merged.Closed)
	assert.Equal(t, "alice", merged.PosterName)
	assert.EqualValues(t, 7, merged.PosterID)
	assert.Equal(t, "feature", merged.Head.Ref)
	assert.Equal(t, headSHA, merged.Head.SHA)
	assert.Equal(t, "main", merged.Base.Ref)
	assert.Equal(t, forkPoint, merged.Base.SHA, "parent of the oldest PR commit, the target branch has moved on")
	assert.False(t, merged.IsForkPullRequest())
	assert.True(t, merged.EnsuredSafe)

	fork := prs[1]
	assert.Equal(t, "open", fork.State)
	assert.True(t, fork.IsDraft)
	assert.Equal(t, "bob", fork.PosterName)
	assert.True(t, fork.IsForkPullRequest())
	assert.Equal(t, srv.URL+"/scm/~BOB/repo-fork.git", fork.Head.CloneURL)
	assert.Equal(t, "~BOB", fork.Head.OwnerName)
	assert.Equal(t, "patch", fork.Head.Ref)
	assert.Nil(t, fork.Closed)

	declined := prs[2]
	assert.Equal(t, "closed", declined.State)
	assert.False(t, declined.Merged)
	assert.Equal(t, time.UnixMilli(1700002500000), *declined.Closed, "falls back to the update time")
	assert.Empty(t, declined.Head.SHA, "invalid SHA is dropped")
}

// newest first, like the API
const bbsActivitiesJSON = `{"isLastPage":false,"nextPageStart":10,"values":[
{"action":"REOPENED","createdDate":1700000900000,"user":{"id":7,"name":"alice"}},
{"action":"DECLINED","createdDate":1700000800000,"user":{"id":7,"name":"alice"}},
{"action":"UNAPPROVED","createdDate":1700000720000,"user":{"id":10,"name":"dave"}},
{"action":"APPROVED","createdDate":1700000710000,"user":{"id":10,"name":"dave"}},
{"action":"REVIEWED","createdDate":1700000700000,"user":{"id":9,"name":"carol"}},
{"action":"COMMENTED","commentAction":"DELETED","createdDate":1700000650000,"user":{"id":7,"name":"alice"},
 "comment":{"id":16,"text":"deleted","author":{"id":7,"name":"alice"},"createdDate":1700000650000}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000640000,"user":{"id":7,"name":"alice"},
 "comment":{"id":16,"text":"deleted","author":{"id":7,"name":"alice"},"createdDate":1700000640000}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000600000,"user":{"id":7,"name":"alice"},
 "commentAnchor":{"path":"README.md","fileType":"TO"},
 "comment":{"id":15,"text":"file comment","author":{"id":7,"name":"alice"},"createdDate":1700000600000,"updatedDate":1700000600000}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000500000,"user":{"id":7,"name":"alice"},
 "commentAnchor":{"line":5,"lineType":"REMOVED","fileType":"FROM","path":"src/b.go","srcPath":"src/old_b.go","fromHash":"fffffffffffffffffffffffffffffffffffffff0","toHash":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0"},
 "comment":{"id":14,"text":"inline old","author":{"id":7,"name":"alice"},"createdDate":1700000500000,"updatedDate":1700000500000}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000400000,"user":{"id":8,"name":"bob"},
 "commentAnchor":{"line":12,"lineType":"ADDED","fileType":"TO","path":"src/a.go","srcPath":"src/a.go","fromHash":"fffffffffffffffffffffffffffffffffffffff0","toHash":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0"},
 "comment":{"id":12,"text":"inline new","author":{"id":8,"name":"bob"},"createdDate":1700000400000,"updatedDate":1700000400000,
  "comments":[{"id":13,"text":"inline reply","author":{"id":7,"name":"alice"},"createdDate":1700000450000,"updatedDate":1700000450000}]}}
]}`

const bbsActivitiesPage2JSON = `{"isLastPage":true,"values":[
{"action":"APPROVED","createdDate":1700000300000,"user":{"id":8,"name":"bob"}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000100000,"user":{"id":8,"name":"bob"},
 "comment":{"id":10,"text":"general comment","author":{"id":8,"name":"bob","emailAddress":"bob@example.com"},"createdDate":1700000100000,"updatedDate":1700000150000,
  "comments":[{"id":11,"text":"a reply","author":{"id":7,"name":"alice"},"createdDate":1700000200000,"updatedDate":1700000200000}]}},
{"action":"OPENED","createdDate":1700000000000,"user":{"id":7,"name":"alice"}}
]}`

func TestBitbucketServerCommentsAndReviews(t *testing.T) {
	var activityCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/1.0/projects/PROJ/repos/repo/pull-requests/4/activities", r.URL.Path)
		activityCalls++
		if r.URL.Query().Get("start") == "10" {
			_, _ = w.Write([]byte(bbsActivitiesPage2JSON))
			return
		}
		_, _ = w.Write([]byte(bbsActivitiesJSON))
	}))
	defer srv.Close()

	d := NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "", "PROJ", "repo")
	pr := &base.PullRequest{Number: 4, ForeignIndex: 4}

	comments, isEnd, err := d.GetComments(t.Context(), pr)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, comments, 5)
	assert.Equal(t, "general comment", comments[0].Content)
	assert.Equal(t, "bob", comments[0].PosterName)
	assert.Equal(t, "bob@example.com", comments[0].PosterEmail)
	assert.EqualValues(t, 10, comments[0].Index)
	assert.EqualValues(t, 4, comments[0].IssueIndex)
	assert.Equal(t, time.UnixMilli(1700000150000), comments[0].Updated)
	assert.Equal(t, "a reply", comments[1].Content)
	assert.Equal(t, "alice", comments[1].PosterName)
	assert.Equal(t, "Comment on `README.md`:\n\nfile comment", comments[2].Content)
	assert.Equal(t, "close", comments[3].CommentType)
	assert.Equal(t, "reopen", comments[4].CommentType)
	// the deleted comment is skipped, inline comments belong to the reviews
	for _, c := range comments {
		assert.NotContains(t, c.Content, "deleted")
		assert.NotContains(t, c.Content, "inline")
	}

	reviews, err := d.GetReviews(t.Context(), pr)
	require.NoError(t, err)
	assert.Equal(t, 2, activityCalls, "activities are paged and cached for both calls")

	type summary struct {
		reviewer, state, path string
		line                  int
	}
	var got []summary
	for _, r := range reviews {
		s := summary{reviewer: r.ReviewerName, state: r.State}
		if len(r.Comments) > 0 {
			s.path, s.line = r.Comments[0].TreePath, r.Comments[0].Line
		}
		got = append(got, s)
	}
	assert.Equal(t, []summary{
		{"bob", base.ReviewStateApproved, "", 0},
		{"bob", base.ReviewStateCommented, "src/a.go", 12},
		{"alice", base.ReviewStateCommented, "src/a.go", 12},
		{"alice", base.ReviewStateCommented, "src/b.go", -5},
		{"carol", base.ReviewStateChangesRequested, "", 0},
	}, got, "dave's approval was withdrawn")
	assert.Equal(t, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0", reviews[1].Comments[0].CommitID)
	assert.Equal(t, "fffffffffffffffffffffffffffffffffffffff0", reviews[3].Comments[0].CommitID)
	assert.Equal(t, "inline reply", reviews[2].Comments[0].Content)
	assert.EqualValues(t, 4, reviews[0].IssueIndex)
}

// TestBitbucketServerDownloadRepo replays responses recorded from a Bitbucket Server 10.5.1 instance.
// To record again, set BITBUCKETSERVER_LIVE_TOKEN (an HTTP access token with project read permission) and
// optionally BITBUCKETSERVER_LIVE_URL (default http://localhost:7990/bitbucket) of an instance holding the
// project MIG with the repository demo.
func TestBitbucketServerDownloadRepo(t *testing.T) {
	liveToken := os.Getenv("BITBUCKETSERVER_LIVE_TOKEN")
	liveURL := cmp.Or(os.Getenv("BITBUCKETSERVER_LIVE_URL"), "http://localhost:7990/bitbucket")

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestBitbucketServerDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, liveURL, fixtureDir, liveToken != "")

	ctx := t.Context()
	token := cmp.Or(liveToken, "replayed") // a token makes the downloader learn the name of its owner
	d := NewBitbucketServerDownloader(ctx, mockServer.URL, "", "", token, "MIG", "demo")

	repo, err := d.GetRepoInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, &base.Repository{
		Name:          "demo",
		Owner:         "MIG",
		IsPrivate:     true,
		CloneURL:      mockServer.URL + "/scm/MIG/demo.git",
		OriginalURL:   mockServer.URL + "/projects/MIG/repos/demo",
		DefaultBranch: "main",
	}, repo)

	prs, isEnd, err := d.GetPullRequests(ctx, 1, 50)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, prs, 4)

	// open PR from a branch of the same repository
	open := prs[0]
	assert.Equal(t, "Open PR", open.Title)
	assert.Equal(t, "open", open.State)
	assert.False(t, open.IsDraft)
	assert.Equal(t, "admin", open.PosterName)
	assert.Equal(t, "feature", open.Head.Ref)
	assert.Equal(t, "810d0600716f4575691c804b583c1c6a6e6708db", open.Head.SHA)
	assert.Equal(t, "main", open.Base.Ref)
	assert.False(t, open.IsForkPullRequest())

	merged := prs[1]
	assert.Equal(t, "closed", merged.State)
	assert.True(t, merged.Merged)
	assert.Equal(t, "504e78ed1c8fcbc0e55f5f0b6d3c49a0bc1ae861", merged.MergeCommitSHA)
	assert.Equal(t, "a41991381bd1e6944b3422cd1dc933b65c74cf67", merged.Base.SHA)
	assert.NotNil(t, merged.MergedTime)

	declined := prs[2]
	assert.Equal(t, "closed", declined.State)
	assert.False(t, declined.Merged)
	assert.NotNil(t, declined.Closed)

	// draft PR from a private fork
	fork := prs[3]
	assert.True(t, fork.IsDraft)
	assert.Equal(t, "bob", fork.PosterName)
	assert.Equal(t, "forkbranch", fork.Head.Ref)
	assert.Equal(t, "aa537b277722995337c78d4d75018202091c0793", fork.Head.SHA)
	assert.True(t, fork.IsForkPullRequest())
	assert.Equal(t, mockServer.URL+"/scm/~BOB/demo.git", fork.Head.CloneURL)

	comments, _, err := d.GetComments(ctx, open)
	require.NoError(t, err)
	var got []string
	for _, c := range comments {
		got = append(got, c.PosterName+": "+c.Content)
	}
	assert.Equal(t, []string{
		"admin: General comment from admin",
		"bob: Reply from bob",
		"carol: Comment on `a.txt`:\n\nFile level comment",
	}, got)

	reviews, err := d.GetReviews(ctx, open)
	require.NoError(t, err)
	type summary struct {
		reviewer, state, path, content string
		line                           int
		resolved                       bool
	}
	var reviewed []summary
	for _, r := range reviews {
		s := summary{reviewer: r.ReviewerName, state: r.State}
		if len(r.Comments) > 0 {
			c := r.Comments[0]
			s.path, s.line, s.content, s.resolved = c.TreePath, c.Line, c.Content, c.Resolved
		}
		reviewed = append(reviewed, s)
	}
	assert.Equal(t, []summary{
		{"bob", base.ReviewStateCommented, "a.txt", "Inline on added line", 2, false},
		{"admin", base.ReviewStateCommented, "a.txt", "Inline reply by admin", 2, false},
		{"carol", base.ReviewStateCommented, "a.txt", "Inline on removed line", -2, false},
		{"bob", base.ReviewStateApproved, "", "", 0, false},
		{"carol", base.ReviewStateChangesRequested, "", "", 0, false},
		{"bob", base.ReviewStateCommented, "a.txt", "Lines 2-4:\n\nMulti line comment", 4, false},
		{"bob", base.ReviewStateCommented, "a.txt", "Thread to resolve", 6, true},
		{"admin", base.ReviewStateCommented, "a.txt", "reply on resolved thread", 6, false},
	}, reviewed, "the resolved state is only set on the first comment of a thread")

	declinedComments, _, err := d.GetComments(ctx, declined)
	require.NoError(t, err)
	require.Len(t, declinedComments, 2)
	assert.Equal(t, "close", declinedComments[0].CommentType)
	assert.Equal(t, "declining this", declinedComments[1].Content)

	// a token is used to clone, with the name of its owner
	cloneURL, err := d.FormatCloneURL(base.MigrateOptions{AuthToken: token}, repo.CloneURL)
	require.NoError(t, err)
	assert.Contains(t, cloneURL, "admin:")
}

func TestBitbucketServerInlineCommentRange(t *testing.T) {
	var anchor bbsAnchor
	require.NoError(t, json.Unmarshal([]byte(`{"line":9,"fileType":"FROM","path":"b","srcPath":"a",
		"multilineMarker":{"startLine":7},"multilineSpan":{"srcSpanStart":3,"srcSpanEnd":5,"dstSpanStart":7,"dstSpanEnd":9}}`), &anchor))
	c := &bbsComment{ID: 1, Text: "text", ThreadResolved: true}

	// the span of the commented side wins, replies neither get the range nor the resolved state
	rc := c.toReviewComment(&anchor, true)
	assert.Equal(t, "Lines 3-5 (before the change):\n\ntext", rc.Content)
	assert.Equal(t, -9, rc.Line)
	assert.Equal(t, "b", rc.TreePath, "Gitea files old-side comments under the current path as well")
	assert.True(t, rc.Resolved)
	rc = c.toReviewComment(&anchor, false)
	assert.Equal(t, "text", rc.Content)
	assert.False(t, rc.Resolved)

	// without a span the marker and the last line are used, a single line has no range
	require.NoError(t, json.Unmarshal([]byte(`{"line":9,"fileType":"TO","path":"a","multilineMarker":{"startLine":7}}`), &anchor))
	start, end, ok := anchor.lineRange()
	assert.True(t, ok)
	assert.Equal(t, [2]int{7, 9}, [2]int{start, end})
	_, _, ok = (&bbsAnchor{Line: 9, FileType: "TO"}).lineRange()
	assert.False(t, ok)
}

func TestBitbucketServerRateLimit(t *testing.T) {
	var calls int
	retryAfter, limitedUntil := "3", 2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= limitedUntil {
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(bbsRepoJSON))
	}))
	defer srv.Close()

	var waits []time.Duration
	newDownloader := func() *BitbucketServerDownloader {
		d := NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "", "PROJ", "repo")
		d.sleep = func(_ context.Context, wait time.Duration) error {
			waits = append(waits, wait)
			return nil
		}
		return d
	}

	// the server is asked again after the time it names
	var repo bbsRepository
	_, err := newDownloader().callAPI(t.Context(), "/projects/PROJ/repos/repo", nil, &repo)
	require.NoError(t, err)
	assert.Equal(t, "repo", repo.Slug)
	assert.Equal(t, []time.Duration{3 * time.Second, 3 * time.Second}, waits)

	// without a hint the wait grows exponentially, and is capped
	calls, limitedUntil, retryAfter, waits = 0, 2, "", nil
	_, err = newDownloader().callAPI(t.Context(), "/projects/PROJ/repos/repo", nil, &repo)
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, waits)
	assert.Equal(t, time.Minute, bitbucketServerRetryAfter("100000", 1))

	// a server that never recovers is given up on
	calls, limitedUntil, waits = 0, 100, nil
	_, err = newDownloader().callAPI(t.Context(), "/projects/PROJ/repos/repo", nil, &repo)
	var statusErr *bbsStatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusTooManyRequests, statusErr.Status)
	assert.Len(t, waits, bitbucketServerMaxAttempts-1)
	assert.Equal(t, bitbucketServerMaxAttempts, calls)
}
