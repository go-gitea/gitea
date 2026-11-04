// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
