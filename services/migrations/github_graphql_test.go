// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"fmt"
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

// TestGraphQLQueryReactionGating guards the shape that stays under BOTH GitHub
// limits: the 500k node cap and the per-query compute budget. When reactions are
// skipped, none are requested. When wanted, the content query carries only the
// entity-level reactions (one connection) — comment reactions must NOT be nested
// (a 3rd connection level blows the limits), they come from the batched node-id
// pass (attachCommentReactions) instead.
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
		assert.Equal(t, 1, strings.Count(keep, "reactions("), "%s: exactly one reactions connection (entity-level); comment reactions must not be nested", tc.name)
		assert.Contains(t, keep, "reactions(first:100){totalCount", "%s: entity reactions carry totalCount for the overflow sweep", tc.name)
	}
}

func TestIsGraphQLResourceLimited(t *testing.T) {
	rl := fmt.Errorf("github graphql: %w", graphQLError{Type: "RESOURCE_LIMITS_EXCEEDED", Message: "too expensive"})
	assert.True(t, isGraphQLResourceLimited(rl))
	assert.False(t, isGraphQLResourceLimited(fmt.Errorf("github graphql: %w", graphQLError{Type: "RATE_LIMITED"})))
	assert.False(t, isGraphQLResourceLimited(errors.New("plain")))
}

// TestFetchNodeReactionsAdaptiveShrink: a batch that comes back
// RESOURCE_LIMITS_EXCEEDED is halved and retried, not aborted. The stub rejects
// any multi-id batch, so a 2-id request must split into two 1-id requests and
// still return reactions for both.
func TestFetchNodeReactionsAdaptiveShrink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				IDs []string `json:"ids"`
			} `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if len(req.Variables.IDs) > 1 {
			_, _ = io.WriteString(w, `{"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED","message":"too expensive"}]}`)
			return
		}
		id := req.Variables.IDs[0]
		_, _ = io.WriteString(w, `{"data":{"nodes":[{"id":"`+id+`","reactions":{"totalCount":1,"nodes":[{"content":"HEART","user":{"login":"u","databaseId":1}}]}}]}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	got, err := d.fetchNodeReactions(t.Context(), []string{"c1", "c2"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Len(t, got["c1"], 1)
	assert.Equal(t, "heart", got["c1"][0].Content)
	require.Len(t, got["c2"], 1)
}

// TestReactionsWithSweepNoOverflow: when an entity's reaction count fits the
// batched page (totalCount == nodes), reactions (incl. the reacting user) come
// straight from the query with no sweep — so it must not touch a (nil) client.
func TestReactionsWithSweepNoOverflow(t *testing.T) {
	var conn gqlReactionConn
	require.NoError(t, json.Unmarshal([]byte(`{"totalCount":2,"nodes":[
		{"content":"THUMBS_UP","user":{"login":"alice","databaseId":11}},
		{"content":"HEART","user":{"login":"bob","databaseId":22}}]}`), &conn))
	r, err := (&GithubDownloaderV3{}).reactionsWithSweep(t.Context(), "node-id", conn)
	require.NoError(t, err)
	require.Len(t, r, 2)
	assert.Equal(t, "+1", r[0].Content)
	assert.EqualValues(t, 11, r[0].UserID)
	assert.Equal(t, "alice", r[0].UserName)
	assert.Equal(t, "heart", r[1].Content)
	assert.Equal(t, "bob", r[1].UserName)
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
	          "reactions": {"totalCount": 2, "nodes": [{"content": "THUMBS_UP", "user": {"login": "alice", "databaseId": 11}}, {"content": "HEART", "user": {"login": "bob", "databaseId": 22}}]},
	          "comments": {
	            "totalCount": 1,
	            "nodes": [
	              {"id": "IC_node1", "databaseId": 900001, "body": "same here", "createdAt": "2024-01-05T00:00:00Z", "updatedAt": "2024-01-05T00:00:00Z", "author": {"login": "reporter", "databaseId": 111}}
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

	// reactions convert with the enum→content mapping and the reacting user
	reactions := convertGraphQLReactions(node.Reactions)
	require.Len(t, reactions, 2)
	assert.Equal(t, "+1", reactions[0].Content)
	assert.EqualValues(t, 11, reactions[0].UserID)
	assert.Equal(t, "alice", reactions[0].UserName)
	assert.Equal(t, "heart", reactions[1].Content)

	// comments convert and carry the issue number as IssueIndex + remote id as
	// Index. Comment reactions are NOT part of the content query — they come from
	// the batched node-id pass (attachCommentReactions) — so none are set here.
	comments := convertGraphQLComments(node.Number, node.Comments.Nodes)
	require.Len(t, comments, 1)
	assert.EqualValues(t, 42, comments[0].IssueIndex)
	assert.EqualValues(t, 900001, comments[0].Index)
	assert.Equal(t, "reporter", comments[0].PosterName)
	assert.Empty(t, comments[0].Reactions, "comment reactions come from the node-id pass, not the content query")
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

// TestDoGraphQLRetriesTransientFailures: a 502 and a truncated body are
// transient — on a sweep that is hours long and hundreds of requests, one such
// blip must not abort the whole run. Each must be retried until a good
// response arrives.
func TestDoGraphQLRetriesTransientFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		bad  func(w http.ResponseWriter)
	}{
		{"bad gateway", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadGateway)
		}},
		{"truncated body", func(w http.ResponseWriter) {
			w.Header().Set("Content-Length", "1024") // promise more than is sent
			_, _ = io.WriteString(w, `{"data":{"vi`) // cut mid-stream
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) <= 2 {
					tc.bad(w)
					return
				}
				_, _ = io.WriteString(w, `{"data":{"viewer":{"login":"ok"}}}`)
			}))
			defer srv.Close()

			d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
			require.NoError(t, err)

			var out struct {
				Viewer struct {
					Login string `json:"login"`
				} `json:"viewer"`
			}
			require.NoError(t, d.doGraphQL(t.Context(), `query{viewer{login}}`, nil, &out))
			assert.Equal(t, "ok", out.Viewer.Login)
			assert.EqualValues(t, 3, calls.Load(), "two transient failures should be retried, third attempt succeeds")
		})
	}
}

// TestDoGraphQLPageShrink: a deterministic 502 — one specific page whose
// response is too heavy for the backend — never succeeds at the same size, so
// after the transient retry budget is exhausted the page is halved and the
// same cursor re-requested. The stub 502s any request with first > 25.
func TestDoGraphQLPageShrink(t *testing.T) {
	prevBase, prevTransportBase := graphQLTransientRetryBase, retryBaseDelay
	graphQLTransientRetryBase, retryBaseDelay = time.Millisecond, time.Millisecond
	defer func() { graphQLTransientRetryBase, retryBaseDelay = prevBase, prevTransportBase }()

	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				First int `json:"first"`
			} `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		sizes = append(sizes, req.Variables.First)
		if req.Variables.First > 25 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	var resp gqlIssuesResponse
	vars := map[string]any{"owner": "o", "name": "r"}
	require.NoError(t, d.doGraphQLPageShrink(t.Context(), d.graphQLIssuesQuery(), vars, &resp, githubGraphQLPageSize, "issues"))

	// each size is attempted (1 + maxGraphQLTransientRetries) times before the
	// shrink; the request sequence must end at a size that fits
	require.NotEmpty(t, sizes)
	assert.Equal(t, 100, sizes[0])
	assert.Equal(t, 25, sizes[len(sizes)-1])
	assert.Equal(t, 25, vars["first"])
}
