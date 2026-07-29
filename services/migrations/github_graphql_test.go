// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsGraphQLRateLimited(t *testing.T) {
	assert.True(t, isGraphQLRateLimited([]graphQLError{{Type: "RATE_LIMITED", Message: "exceeded"}}))
	assert.True(t, isGraphQLRateLimited([]graphQLError{{Type: "NOT_FOUND"}, {Type: "RATE_LIMITED"}}))
	assert.False(t, isGraphQLRateLimited([]graphQLError{{Type: "NOT_FOUND"}}))
	assert.False(t, isGraphQLRateLimited(nil))
}

func TestGraphQLRateResetWait(t *testing.T) {
	// Retry-After (seconds) takes precedence
	h := http.Header{}
	h.Set("Retry-After", "30")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	assert.Equal(t, 30*time.Second, graphQLRateResetWait(h))

	// X-RateLimit-Reset (unix epoch, in the future) when no Retry-After
	h = http.Header{}
	h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(90*time.Second).Unix(), 10))
	got := graphQLRateResetWait(h)
	assert.Greater(t, got, 60*time.Second)
	assert.LessOrEqual(t, got, 90*time.Second)

	// a reset already in the past, and no headers at all, both fall back
	past := http.Header{}
	past.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10))
	assert.Equal(t, graphQLRateResetFallback, graphQLRateResetWait(past))
	assert.Equal(t, graphQLRateResetFallback, graphQLRateResetWait(http.Header{}))

	// a bogus far-future reset is capped
	huge := http.Header{}
	huge.Set("Retry-After", "999999")
	assert.Equal(t, graphQLRateResetCap, graphQLRateResetWait(huge))
}

// TestDoGraphQLRetriesOnRateLimited proves the fix for the abort-on-RATE_LIMITED
// bug: a RATE_LIMITED response is waited out (per the Retry-After header) and the
// request retried in place, instead of erroring and aborting the whole sync.
func TestDoGraphQLRetriesOnRateLimited(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1") // 1s so the test stays fast
			io.WriteString(w, `{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`)
			return
		}
		io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	var out struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, d.doGraphQL(t.Context(), "query{}", nil, &out))
	assert.True(t, out.OK)
	assert.EqualValues(t, 2, calls.Load(), "RATE_LIMITED should be waited out and the request retried once")
}

// TestGraphQLQueryReactionGating guards the MAX_NODE_LIMIT_EXCEEDED fix: reactions
// must never be nested under comments (issues×comments×reactions = 1,000,000 nodes,
// which GitHub rejects), and must be omitted entirely when SkipReactions is set.
// The reactions COUNT is the tell — with reactions wanted it must appear exactly
// once (the entity level); a second occurrence means the fatal comment-nesting
// regressed.
func TestGraphQLQueryReactionGating(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query func(*GithubDownloaderV3) string
	}{
		{"issues", (*GithubDownloaderV3).graphQLIssuesQuery},
		{"pulls", (*GithubDownloaderV3).graphQLPullRequestsQuery},
	} {
		skip := tc.query(&GithubDownloaderV3{SkipReactions: true})
		keep := tc.query(&GithubDownloaderV3{SkipReactions: false})
		assert.Equal(t, 0, strings.Count(skip, "reactions"), "%s: SkipReactions must omit all reactions", tc.name)
		assert.Equal(t, 1, strings.Count(keep, "reactions"), "%s: reactions only at the entity level, never nested under comments", tc.name)
		// the comments connection must carry no nested reactions in either mode
		assert.NotContains(t, skip, "comments(first:100){totalCount nodes{databaseId body createdAt updatedAt author{login ... on User{databaseId}} reactions", tc.name)
	}
}

func TestGraphQLEndpoint(t *testing.T) {
	assert.Equal(t, "https://api.github.com/graphql", (&GithubDownloaderV3{baseURL: "https://github.com"}).graphQLEndpoint())
	assert.Equal(t, "https://api.github.com/graphql", (&GithubDownloaderV3{baseURL: ""}).graphQLEndpoint())
	assert.Equal(t, "https://ghe.example.com/api/graphql", (&GithubDownloaderV3{baseURL: "https://ghe.example.com"}).graphQLEndpoint())
	assert.Equal(t, "https://ghe.example.com/api/graphql", (&GithubDownloaderV3{baseURL: "https://ghe.example.com/"}).graphQLEndpoint())
}

func TestGraphQLReactionContent(t *testing.T) {
	assert.Equal(t, "+1", graphQLReactionContent("THUMBS_UP"))
	assert.Equal(t, "-1", graphQLReactionContent("THUMBS_DOWN"))
	assert.Equal(t, "heart", graphQLReactionContent("HEART"))
	assert.Equal(t, "rocket", graphQLReactionContent("ROCKET"))
	// unknown enum falls back to lower-case
	assert.Equal(t, "star", graphQLReactionContent("STAR"))
}

// TestGraphQLIssuesResponseParsing validates that the response structs match the
// shape of graphQLIssuesQuery and that a page parses into the expected entities.
func TestGraphQLIssuesResponseParsing(t *testing.T) {
	const sample = `{
	  "repository": {
	    "issues": {
	      "pageInfo": {"hasNextPage": true, "endCursor": "Y3Vyc29yOjEwMA=="},
	      "nodes": [
	        {
	          "number": 42,
	          "title": "a bug",
	          "body": "it broke",
	          "state": "CLOSED",
	          "createdAt": "2024-01-02T03:04:05Z",
	          "updatedAt": "2024-02-03T04:05:06Z",
	          "closedAt": "2024-02-03T04:05:06Z",
	          "locked": true,
	          "author": {"login": "octocat", "databaseId": 583231},
	          "milestone": {"title": "v1.0"},
	          "labels": {"nodes": [{"name": "bug", "color": "d73a4a", "description": "defect"}]},
	          "assignees": {"nodes": [{"login": "maintainer"}]},
	          "reactions": {"nodes": [{"content": "THUMBS_UP"}, {"content": "HEART"}]},
	          "comments": {
	            "totalCount": 1,
	            "nodes": [
	              {"databaseId": 900001, "body": "same here", "createdAt": "2024-01-05T00:00:00Z", "updatedAt": "2024-01-05T00:00:00Z", "author": {"login": "reporter", "databaseId": 111}, "reactions": {"nodes": [{"content": "ROCKET"}]}}
	            ]
	          }
	        }
	      ]
	    }
	  },
	  "rateLimit": {"cost": 1, "remaining": 4999, "resetAt": "2024-01-01T00:00:00Z"}
	}`

	var resp gqlIssuesResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))

	assert.True(t, resp.Repository.Issues.PageInfo.HasNextPage)
	assert.Equal(t, "Y3Vyc29yOjEwMA==", resp.Repository.Issues.PageInfo.EndCursor)
	assert.Equal(t, 4999, resp.RateLimit.Remaining)
	require.Len(t, resp.Repository.Issues.Nodes, 1)

	node := resp.Repository.Issues.Nodes[0]
	assert.EqualValues(t, 42, node.Number)
	assert.Equal(t, "CLOSED", node.State)
	assert.Equal(t, "octocat", node.Author.Login)
	assert.EqualValues(t, 583231, node.Author.DatabaseID)
	assert.True(t, node.Locked)
	require.NotNil(t, node.Milestone)
	assert.Equal(t, "v1.0", node.Milestone.Title)

	// reactions convert with the enum→content mapping
	reactions := convertGraphQLReactions(node.Reactions)
	require.Len(t, reactions, 2)
	assert.Equal(t, "+1", reactions[0].Content)
	assert.Equal(t, "heart", reactions[1].Content)

	// comments convert and carry the issue number as IssueIndex + remote id as Index
	comments := convertGraphQLComments(node.Number, node.Comments.Nodes)
	require.Len(t, comments, 1)
	assert.EqualValues(t, 42, comments[0].IssueIndex)
	assert.EqualValues(t, 900001, comments[0].Index)
	assert.Equal(t, "reporter", comments[0].PosterName)
	require.Len(t, comments[0].Reactions, 1)
	assert.Equal(t, "rocket", comments[0].Reactions[0].Content)
}

// TestGetCachedComments checks the cache-paging that serves GraphQL-fetched
// comments to the framework's comment phase.
func TestGetCachedComments(t *testing.T) {
	g := &GithubDownloaderV3{
		useGraphQL: true,
		gqlComments: map[int64][]*base.Comment{
			2: {{IssueIndex: 2, Index: 20}, {IssueIndex: 2, Index: 21}},
			1: {{IssueIndex: 1, Index: 10}},
		},
	}
	// page 1, size 2: ordered by issue number (1 then 2)
	page1, isEnd, err := g.getCachedComments(1, 2)
	require.NoError(t, err)
	assert.False(t, isEnd)
	require.Len(t, page1, 2)
	assert.EqualValues(t, 10, page1[0].Index)
	assert.EqualValues(t, 20, page1[1].Index)

	page2, isEnd, err := g.getCachedComments(2, 2)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, page2, 1)
	assert.EqualValues(t, 21, page2[0].Index)

	// past the end returns empty + end
	page3, isEnd, err := g.getCachedComments(3, 2)
	require.NoError(t, err)
	assert.True(t, isEnd)
	assert.Empty(t, page3)
}
