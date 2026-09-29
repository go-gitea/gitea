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
	"sync"
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

// TestGetCommentsServesAndDropsCache: the GraphQL path serves each entity's
// comments from the sweep's cache and drops them as it goes, so the cache holds
// only the page being imported instead of the whole repository's comments (which
// is why SupportGetRepoComments says no for this path).
func TestGetCommentsServesAndDropsCache(t *testing.T) {
	g := &GithubDownloaderV3{
		useGraphQL: true,
		gqlComments: map[int64][]*base.Comment{
			1: {{IssueIndex: 1, Index: 10}},
			2: {{IssueIndex: 2, Index: 20}, {IssueIndex: 2, Index: 21}},
		},
	}
	assert.False(t, g.SupportGetRepoComments(), "the GraphQL path must not buffer every comment for a repo-wide phase")

	got, _, err := g.GetComments(t.Context(), &base.Issue{Number: 2, ForeignIndex: 2})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.EqualValues(t, 20, got[0].Index)
	assert.NotContains(t, g.gqlComments, int64(2), "served comments must be dropped from the cache")
	assert.Contains(t, g.gqlComments, int64(1), "other entities stay cached until served")

	// an entity with no comments is served as empty, not an error
	none, _, err := g.GetComments(t.Context(), &base.Issue{Number: 9, ForeignIndex: 9})
	require.NoError(t, err)
	assert.Empty(t, none)
}

// TestGetReviewsServesAndDropsCache mirrors the comment cache for reviews.
func TestGetReviewsServesAndDropsCache(t *testing.T) {
	g := &GithubDownloaderV3{
		useGraphQL: true,
		gqlReviews: map[int64][]*base.Review{7: {{ID: 900, IssueIndex: 7}}},
	}
	got, err := g.GetReviews(t.Context(), &base.PullRequest{Number: 7, ForeignIndex: 7})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.EqualValues(t, 900, got[0].ID)
	assert.NotContains(t, g.gqlReviews, int64(7), "served reviews must be dropped from the cache")
}

// TestGraphQLPageSize: the connection size honours the batch size the framework
// asked for (it is derived from the uploader's max batch insert size, so a bigger
// page overflows the DB's bind-parameter limit), capped at what the query affords.
func TestGraphQLPageSize(t *testing.T) {
	assert.Equal(t, 41, graphQLPageSize(41, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(500, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(0, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPRPageSize, graphQLPageSize(41, githubGraphQLPRPageSize))
}

// TestGraphQLQueriesSelectNonUserActorIDs guards the id of every non-User author:
// databaseId is not on the Actor interface, so selecting it only `... on User`
// silently drops the numeric id of every bot, organization and mannequin — which
// collapses them all onto original_author_id 0.
func TestGraphQLQueriesSelectNonUserActorIDs(t *testing.T) {
	g := &GithubDownloaderV3{}
	for name, q := range map[string]string{
		"issues":   g.graphQLIssuesQuery(),
		"pulls":    g.graphQLPullRequestsQuery(),
		"timeline": graphQLTimelineBatchQuery(),
	} {
		for _, actorType := range []string{"User", "Bot", "Organization", "Mannequin"} {
			assert.Contains(t, q, "... on "+actorType+"{databaseId}", "%s query must select the %s author id", name, actorType)
		}
	}
}

// TestGraphQLQueriesDetectSideConnectionOverflow: every connection the content
// query caps must select totalCount, or an entity with more labels/assignees/
// reviewers than the cap imports truncated with no warning.
func TestGraphQLQueriesDetectSideConnectionOverflow(t *testing.T) {
	g := &GithubDownloaderV3{}
	issues, pulls := g.graphQLIssuesQuery(), g.graphQLPullRequestsQuery()
	for _, conn := range []string{"labels", "assignees"} {
		assert.Regexp(t, conn+`\(first:\d+\)\{totalCount`, issues, "issue %s must carry totalCount", conn)
		assert.Regexp(t, conn+`\(first:\d+\)\{totalCount`, pulls, "PR %s must carry totalCount", conn)
	}
	assert.Regexp(t, `reviewRequests\(first:\d+\)\{totalCount`, pulls)
}

// TestGraphQLContentQueriesPageByCreation: the content queries walk an IMMUTABLE
// sort key. Cursor pagination over UPDATED_AT loses rows — an entity touched
// mid-sweep re-sorts behind the cursor, where no later page ever returns it.
func TestGraphQLContentQueriesPageByCreation(t *testing.T) {
	g := &GithubDownloaderV3{}
	for name, q := range map[string]string{"issues": g.graphQLIssuesQuery(), "pulls": g.graphQLPullRequestsQuery()} {
		assert.Contains(t, q, "orderBy:{field:CREATED_AT,direction:ASC}", "%s must page by creation order", name)
		assert.NotContains(t, q, "UPDATED_AT", "%s must not paginate over a mutable sort key", name)
	}
}

// TestLabelsWithSweepOverflow / TestAssigneesWithSweepOverflow: a truncated side
// connection is swept by node id instead of silently importing short.
func TestLabelsWithSweepOverflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"node":{"conn":{"pageInfo":{"hasNextPage":false},"nodes":[
			{"name":"a","color":"111111"},{"name":"b","color":"222222"},{"name":"c","color":"333333"}]}}}}`)
	}))
	defer srv.Close()
	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	var conn gqlLabelConn
	require.NoError(t, json.Unmarshal([]byte(`{"totalCount":3,"nodes":[{"name":"a","color":"111111"}]}`), &conn))
	labels, err := d.labelsWithSweep(t.Context(), "I_node", conn)
	require.NoError(t, err)
	require.Len(t, labels, 3, "an issue with more labels than the page size must be swept, not truncated")
	assert.Equal(t, "c", labels[2].Name)

	// contained: no sweep, so the (unreachable) server is never asked
	conn.TotalCount = 1
	labels, err = d.labelsWithSweep(t.Context(), "", conn)
	require.NoError(t, err)
	require.Len(t, labels, 1)
}

func TestAssigneesWithSweepOverflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"node":{"conn":{"pageInfo":{"hasNextPage":false},"nodes":[{"login":"a"},{"login":"b"}]}}}}`)
	}))
	defer srv.Close()
	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	got, err := d.assigneesWithSweep(t.Context(), "I_node", gqlAssigneeConn{TotalCount: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got)
}

// TestDoGraphQLRetriesTransientFailures: a 502 and a truncated body are
// transient — on a sweep that is hours long and hundreds of requests, one such
// blip must not abort the whole run. Each must be retried until a good response
// arrives. The 502 is retried by the transport (http_client.go), the truncated
// body by doGraphQL, since the transport cannot replay a body that died after
// the response headers.
func TestDoGraphQLRetriesTransientFailures(t *testing.T) {
	prevBase, prevTransportBase := graphQLTransientRetryBase, retryBaseDelay
	graphQLTransientRetryBase, retryBaseDelay = time.Millisecond, time.Millisecond
	defer func() { graphQLTransientRetryBase, retryBaseDelay = prevBase, prevTransportBase }()

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

// TestDoGraphQLDoesNotReRetryTransportFailures: retryTransport owns 5xx and
// network retries, so doGraphQL must not stack a second budget on top of it —
// four nested retry layers turn one bad page into hours of backoff. A 5xx that
// survives the transport is reported as transient-exhausted so the page-shrink
// escalation starts immediately.
func TestDoGraphQLDoesNotReRetryTransportFailures(t *testing.T) {
	prevTransportBase := retryBaseDelay
	retryBaseDelay = time.Millisecond
	defer func() { retryBaseDelay = prevTransportBase }()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	err = d.doGraphQL(t.Context(), `query{viewer{login}}`, nil, &struct{}{})
	require.ErrorIs(t, err, errGraphQLTransientExhausted)
	assert.ErrorContains(t, err, "502")
	assert.EqualValues(t, retryMaxRetries+1, calls.Load(), "only the transport's budget may be spent on a 5xx")
}

// TestDoGraphQLWaitsOutSecondaryRateLimit proves the fix for aborting on a
// secondary rate limit: GitHub answers those with 403/429 and not always a
// Retry-After, so the transport passes them straight through. Treating them as
// fatal aborted the whole metadata import — exactly what this path exists to
// prevent.
func TestDoGraphQLWaitsOutSecondaryRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		respond func(w http.ResponseWriter)
	}{
		{"429 without Retry-After", func(w http.ResponseWriter) {
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
			w.WriteHeader(http.StatusTooManyRequests)
		}},
		{"403 with Retry-After", func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusForbidden)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					tc.respond(w)
					return
				}
				_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
			}))
			defer srv.Close()

			d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
			require.NoError(t, err)

			var out struct {
				OK bool `json:"ok"`
			}
			require.NoError(t, d.doGraphQL(t.Context(), "query{}", nil, &out))
			assert.True(t, out.OK)
		})
	}
}

// TestDoGraphQLFatalStatusQuotesBody: a permission 403 is not a throttle and must
// fail at once, carrying GitHub's explanation — without the body the log says only
// "unexpected status" and nothing actionable.
func TestDoGraphQLFatalStatusQuotesBody(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"Resource not accessible by integration"}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	err = d.doGraphQL(t.Context(), "query{}", nil, &struct{}{})
	require.Error(t, err)
	assert.NotErrorIs(t, err, errGraphQLTransientExhausted)
	assert.ErrorContains(t, err, "Resource not accessible by integration")
	assert.EqualValues(t, 1, calls.Load(), "a permission 403 must not be waited out")
}

// TestGetIssuesGraphQLKeepsCursorOnPageError proves the fix for a whole page of
// issues going missing: the framework retries a failed page with the SAME page
// number, so the cursor may only advance once the page has fully converted. Here
// the reaction sweep fails mid-page; the retry must re-request this page, not the
// next one.
func TestGetIssuesGraphQLKeepsCursorOnPageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "Reactable") {
			// the overflow sweep fails deterministically
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"message":"nope"}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"repository":{"issues":{
			"pageInfo":{"hasNextPage":true,"endCursor":"PAGE2"},
			"nodes":[{"id":"I_1","number":1,"title":"t","state":"OPEN",
			  "reactions":{"totalCount":300,"nodes":[{"content":"HEART","user":{"login":"u","databaseId":1}}]}}]}}}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)
	d.useGraphQL = true

	_, _, err = d.getIssuesGraphQL(t.Context(), 1, 10)
	require.Error(t, err)
	assert.Empty(t, d.gqlIssuesCursor, "a page that failed mid-conversion must be re-requested, not skipped")
}

// TestDoGraphQLPageShrink: a deterministic 502 — one specific page whose
// response is too heavy for the backend — never succeeds at the same size, so
// after the transient retry budget is exhausted the page is halved and the
// same cursor re-requested. The stub 502s any request with first > 25.
func TestDoGraphQLPageShrink(t *testing.T) {
	prevBase, prevTransportBase := graphQLTransientRetryBase, retryBaseDelay
	graphQLTransientRetryBase, retryBaseDelay = time.Millisecond, time.Millisecond
	defer func() { graphQLTransientRetryBase, retryBaseDelay = prevBase, prevTransportBase }()

	var (
		mu    sync.Mutex // the handler runs on the server's goroutines
		sizes []int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				First int `json:"first"`
			} `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		mu.Lock()
		sizes = append(sizes, req.Variables.First)
		mu.Unlock()
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

	// each size is attempted (1 + retryMaxRetries) times — the transport's budget,
	// not a second one on top of it — before the shrink; the request sequence must
	// end at a size that fits
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, sizes)
	assert.Equal(t, 100, sizes[0])
	assert.Equal(t, 25, sizes[len(sizes)-1])
	assert.Equal(t, 25, vars["first"])
	assert.Len(t, sizes, 2*(retryMaxRetries+1)+1, "sizes 100 and 50 exhaust the transport budget, 25 succeeds first try")
}
