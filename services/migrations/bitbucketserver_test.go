// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
}

const bbsRepoJSON = `{"slug":"repo","name":"Repo","description":"a test repo","public":false,"project":{"key":"PROJ"}}`

func bbsMockServer(t *testing.T, handlers map[string]string, requests *[]*http.Request) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			*requests = append(*requests, r)
		}
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			if body, ok := handlers[key+"?"+r.URL.Query().Get("start")]; ok {
				_, _ = w.Write([]byte(body))
				return
			}
		}
		body, ok := handlers[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBitbucketServerRepoInfo(t *testing.T) {
	const api = "/rest/api/1.0/projects/PROJ/repos/repo"
	var requests []*http.Request
	// no default branch: an empty repository answers 404
	srv := bbsMockServer(t, map[string]string{api: bbsRepoJSON}, &requests)

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

	// releases are not provided, tags are synced from git
	_, err = d.GetReleases(t.Context())
	assert.True(t, base.IsErrNotSupported(err))
}

func TestBitbucketServerBasicAuth(t *testing.T) {
	var requests []*http.Request
	srv := bbsMockServer(t, map[string]string{
		"/rest/api/1.0/projects/~jdoe/repos/repo":                bbsRepoJSON,
		"/rest/api/1.0/projects/~jdoe/repos/repo/default-branch": `{"displayId":"main"}`,
	}, &requests)

	d := NewBitbucketServerDownloader(t.Context(), srv.URL, "u", "p", "", "~jdoe", "repo")
	repo, err := d.GetRepoInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "main", repo.DefaultBranch)
	assert.Equal(t, srv.URL+"/users/jdoe/repos/repo", repo.OriginalURL)
	user, pass, ok := requests[0].BasicAuth()
	assert.True(t, ok)
	assert.Equal(t, "u", user)
	assert.Equal(t, "p", pass)

	// a failing API call (here: the default branch is forbidden) must not be swallowed
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/default-branch") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(bbsRepoJSON))
	}))
	defer srv2.Close()
	_, err = NewBitbucketServerDownloader(t.Context(), srv2.URL, "", "", "", "PROJ", "repo").GetRepoInfo(t.Context())
	assert.ErrorContains(t, err, "status 401")
}

const bbsPullRequestsJSON = `{"isLastPage":true,"values":[
{"id":1,"title":"merged","description":"d1","state":"MERGED","createdDate":1700000000000,"updatedDate":1700000300000,"closedDate":1700000200000,
 "author":{"user":{"id":7,"name":"alice","emailAddress":"alice@example.com"}},
 "fromRef":{"displayId":"feature","latestCommit":"%[1]s","repository":{"slug":"repo","project":{"key":"PROJ"}}},
 "toRef":{"displayId":"main","latestCommit":"%[2]s","repository":{"slug":"repo","project":{"key":"PROJ"}}},
 "properties":{"mergeCommit":{"id":"%[3]s"}}},
{"id":2,"title":"fork pr","state":"OPEN","draft":true,"createdDate":1700001000000,"updatedDate":1700001000000,
 "author":{"user":{"id":8,"slug":"bob"}},
 "fromRef":{"displayId":"patch","latestCommit":"%[1]s","repository":{"slug":"repo-fork","project":{"key":"~BOB"}}},
 "toRef":{"displayId":"main","latestCommit":"%[2]s","repository":{"slug":"repo","project":{"key":"PROJ"}}}},
{"id":3,"title":"declined","state":"DECLINED","createdDate":1700002000000,"updatedDate":1700002500000,
 "author":{"user":{"id":7,"name":"alice"}},
 "fromRef":{"displayId":"x","latestCommit":"not-a-sha","repository":{"slug":"repo","project":{"key":"PROJ"}}},
 "toRef":{"displayId":"main","latestCommit":"%[2]s","repository":{"slug":"repo","project":{"key":"PROJ"}}}}
]}`

func TestBitbucketServerPullRequests(t *testing.T) {
	headSHA, baseSHA, mergeSHA := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	const api = "/rest/api/1.0/projects/PROJ/repos/repo"
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(strings.NewReplacer("%[1]s", headSHA, "%[2]s", baseSHA, "%[3]s", mergeSHA).Replace(bbsPullRequestsJSON)))
		assert.Equal(t, api+"/pull-requests", r.URL.Path)
	}))
	defer srv.Close()

	d := NewBitbucketServerDownloader(t.Context(), srv.URL, "", "", "", "PROJ", "repo")
	prs, isEnd, err := d.GetPullRequests(t.Context(), 2, 500)
	require.NoError(t, err)
	assert.True(t, isEnd)
	assert.Equal(t, "limit=100&order=oldest&start=100&state=ALL", query, "page size is capped and the offset follows the page")
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
	assert.Empty(t, merged.Base.SHA, "merge base is computed by the uploader")
	assert.False(t, merged.IsForkPullRequest())
	assert.True(t, merged.EnsuredSafe)

	fork := prs[1]
	assert.Equal(t, "open", fork.State)
	assert.True(t, fork.IsDraft)
	assert.Equal(t, "bob", fork.PosterName)
	assert.True(t, fork.IsForkPullRequest())
	assert.Equal(t, srv.URL+"/scm/~BOB/repo-fork.git", fork.Head.CloneURL)
	assert.Equal(t, "~BOB", fork.Head.OwnerName)
	assert.Nil(t, fork.Closed)

	declined := prs[2]
	assert.Equal(t, "closed", declined.State)
	assert.False(t, declined.Merged)
	assert.Equal(t, time.UnixMilli(1700002500000), *declined.Closed, "falls back to the update time")
	assert.Empty(t, declined.Head.SHA, "invalid SHA is dropped")
}

const bbsActivitiesJSON = `{"isLastPage":false,"nextPageStart":3,"values":[
{"action":"OPENED","createdDate":1700000000000,"user":{"id":7,"name":"alice"}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000100000,"user":{"id":8,"name":"bob"},
 "comment":{"id":10,"text":"general comment","author":{"id":8,"name":"bob","emailAddress":"bob@example.com"},"createdDate":1700000100000,"updatedDate":1700000150000,
  "comments":[{"id":11,"text":"a reply","author":{"id":7,"name":"alice"},"createdDate":1700000200000,"updatedDate":1700000200000}]}},
{"action":"APPROVED","createdDate":1700000300000,"user":{"id":8,"name":"bob"}}
]}`

const bbsActivitiesPage2JSON = `{"isLastPage":true,"values":[
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000400000,"user":{"id":8,"name":"bob"},
 "commentAnchor":{"line":12,"lineType":"ADDED","fileType":"TO","path":"src/a.go","srcPath":"src/a.go","fromHash":"` + "fffffffffffffffffffffffffffffffffffffff0" + `","toHash":"` + "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0" + `"},
 "comment":{"id":12,"text":"inline new","author":{"id":8,"name":"bob"},"createdDate":1700000400000,"updatedDate":1700000400000,
  "comments":[{"id":13,"text":"inline reply","author":{"id":7,"name":"alice"},"createdDate":1700000450000,"updatedDate":1700000450000}]}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000500000,"user":{"id":7,"name":"alice"},
 "commentAnchor":{"line":5,"lineType":"REMOVED","fileType":"FROM","path":"src/b.go","srcPath":"src/old_b.go","fromHash":"` + "fffffffffffffffffffffffffffffffffffffff0" + `","toHash":"` + "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0" + `"},
 "comment":{"id":14,"text":"inline old","author":{"id":7,"name":"alice"},"createdDate":1700000500000,"updatedDate":1700000500000}},
{"action":"COMMENTED","commentAction":"ADDED","createdDate":1700000600000,"user":{"id":7,"name":"alice"},
 "commentAnchor":{"path":"README.md","fileType":"TO"},
 "comment":{"id":15,"text":"file comment","author":{"id":7,"name":"alice"},"createdDate":1700000600000,"updatedDate":1700000600000}},
{"action":"COMMENTED","commentAction":"DELETED","createdDate":1700000650000,"user":{"id":7,"name":"alice"},
 "comment":{"id":16,"text":"deleted","author":{"id":7,"name":"alice"},"createdDate":1700000650000}},
{"action":"REVIEWED","createdDate":1700000700000,"user":{"id":9,"name":"carol"}},
{"action":"APPROVED","createdDate":1700000710000,"user":{"id":10,"name":"dave"}},
{"action":"UNAPPROVED","createdDate":1700000720000,"user":{"id":10,"name":"dave"}},
{"action":"DECLINED","createdDate":1700000800000,"user":{"id":7,"name":"alice"}},
{"action":"REOPENED","createdDate":1700000900000,"user":{"id":7,"name":"alice"}}
]}`

func TestBitbucketServerCommentsAndReviews(t *testing.T) {
	var activityCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/1.0/projects/PROJ/repos/repo/pull-requests/4/activities", r.URL.Path)
		activityCalls++
		if r.URL.Query().Get("start") == "3" {
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
		{"alice", base.ReviewStateCommented, "src/old_b.go", -5},
		{"carol", base.ReviewStateChangesRequested, "", 0},
	}, got, "dave's approval was withdrawn")
	assert.Equal(t, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0", reviews[1].Comments[0].CommitID)
	assert.Equal(t, "fffffffffffffffffffffffffffffffffffffff0", reviews[3].Comments[0].CommitID)
	assert.Equal(t, "inline reply", reviews[2].Comments[0].Content)
	assert.EqualValues(t, 4, reviews[0].IssueIndex)
}
