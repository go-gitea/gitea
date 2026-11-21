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

func TestGraphQLIncrementalQueryOrder(t *testing.T) {
	g := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{}}
	assert.Contains(t, g.graphQLIssuesQuery(), "orderBy:{field:CREATED_AT,direction:ASC}")
	assert.NotContains(t, g.graphQLIssuesQuery(), "$since")

	g.since = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	issues := g.graphQLIssuesQuery()
	assert.Contains(t, issues, "$since:DateTime!")
	assert.Contains(t, issues, "orderBy:{field:UPDATED_AT,direction:ASC},filterBy:{since:$since}")
	// pullRequests has no since filter: the light listing walks newest first, the content is fetched by number
	assert.Contains(t, graphQLPullRequestUpdatesQuery, "orderBy:{field:UPDATED_AT,direction:DESC}")
	assert.NotContains(t, graphQLPullRequestUpdatesQuery, "$since")
	byNumber := g.graphQLPullRequestsByNumberQuery([]int64{7, 42})
	assert.Contains(t, byNumber, "pr7:pullRequest(number:7){")
	assert.Contains(t, byNumber, "pr42:pullRequest(number:42){")
	assert.Contains(t, byNumber, "headRefName")
}

func TestOnlyGitHubSupportsSyncing(t *testing.T) {
	assert.True(t, (&GithubDownloaderV3{}).SupportSyncing())
	assert.False(t, (&GitBucketDownloader{GithubDownloaderV3: &GithubDownloaderV3{}}).SupportSyncing())
}

func TestGithubGetPullRequestsByNumberSkipsDeleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/repos/o/r/pulls/1", "/api/v3/repos/o/r/pulls/3":
			number := strings.TrimPrefix(r.URL.Path, "/api/v3/repos/o/r/pulls/")
			_, _ = w.Write([]byte(`{"number":` + number + `,"title":"pull ` + number + `","state":"open","user":{"id":1,"login":"user"},` +
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z",` +
				`"head":{"ref":"feature","sha":"0123456789abcdef0123456789abcdef01234567","repo":{"name":"r","clone_url":"https://example.com/o/r.git","owner":{"login":"o"}}},` +
				`"base":{"ref":"main","sha":"89abcdef0123456789abcdef0123456789abcdef","repo":{"name":"r","owner":{"login":"o"}}}}`))
		case "/api/v3/repos/o/r/issues/1/reactions", "/api/v3/repos/o/r/issues/3/reactions":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
	require.NoError(t, err)
	prs, err := downloader.GetPullRequestsByNumber(t.Context(), []int64{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, prs, 2)
	assert.EqualValues(t, 1, prs[0].Number)
	assert.EqualValues(t, 3, prs[1].Number)
}

func TestGithubGetCommentedEntities(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "2026-01-01T00:00:00Z", r.URL.Query().Get("since"))
		assert.Equal(t, "updated", r.URL.Query().Get("sort"))
		switch r.URL.Path + "?page=" + r.URL.Query().Get("page") {
		case "/api/v3/repos/o/r/issues/comments?page=":
			w.Header().Set("Link", `<`+serverURL+`/api/v3/repos/o/r/issues/comments?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[` +
				`{"issue_url":"https://api.github.com/repos/o/r/issues/5","html_url":"https://github.com/o/r/issues/5#issuecomment-1","updated_at":"2026-01-02T00:00:00Z"},` +
				`{"issue_url":"https://api.github.com/repos/o/r/issues/6","html_url":"https://github.com/o/r/pull/6#issuecomment-2","updated_at":"2026-01-03T00:00:00Z"}]`))
		case "/api/v3/repos/o/r/issues/comments?page=2":
			_, _ = w.Write([]byte(`[{"issue_url":"https://api.github.com/repos/o/r/issues/5","html_url":"https://github.com/o/r/issues/5#issuecomment-3","updated_at":"2026-01-05T00:00:00Z"}]`))
		case "/api/v3/repos/o/r/pulls/comments?page=":
			_, _ = w.Write([]byte(`[{"pull_request_url":"https://api.github.com/repos/o/r/pulls/7","updated_at":"2026-01-04T00:00:00Z"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
	require.NoError(t, err)
	entities, err := downloader.GetCommentedEntities(t.Context(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	assert.Equal(t, []base.CommentedEntity{
		{Number: 6, IsPull: true, Since: day(3), Until: day(3)},
		{Number: 7, IsPull: true, Since: day(4), Until: day(4)},
		{Number: 5, IsPull: false, Since: day(2), Until: day(5)},
	}, entities)

	entities, err = downloader.GetCommentedEntities(t.Context(), time.Time{})
	require.NoError(t, err)
	assert.Empty(t, entities, "a first sync lists nothing")
}

func TestGithubGetIssuesByNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/repos/o/r/issues/5":
			_, _ = w.Write([]byte(`{"number":5,"title":"issue 5","state":"open","user":{"id":1,"login":"user"},` +
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}`))
		case "/api/v3/repos/o/r/issues/6":
			_, _ = w.Write([]byte(`{"number":6,"title":"pull 6","state":"open","user":{"id":1,"login":"user"},"pull_request":{"url":"x"},` +
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}`))
		case "/api/v3/repos/o/r/issues/5/reactions":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
	require.NoError(t, err)
	issues, err := downloader.GetIssuesByNumber(t.Context(), []int64{5, 6, 7})
	require.NoError(t, err)
	require.Len(t, issues, 1) // 6 is a pull request, 7 is gone
	assert.EqualValues(t, 5, issues[0].Number)
	assert.Equal(t, "issue 5", issues[0].Title)
}
