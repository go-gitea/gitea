// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
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
	"gitea.dev/modules/test"

	"github.com/google/go-github/v92/github"
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

	// with primary quota left the throttle is a secondary limit: its reset header
	// belongs to the primary window, so the fallback applies instead
	secondary := http.Header{}
	secondary.Set("X-RateLimit-Remaining", "4321")
	secondary.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	assert.Equal(t, graphQLRateResetFallback, graphQLRateResetWait(secondary))

	// a bogus far-future reset is capped
	huge := http.Header{}
	huge.Set("Retry-After", "999999")
	assert.Equal(t, graphQLRateResetCap, graphQLRateResetWait(huge))
}

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

// comment reactions nested under comments would exceed GitHub's 500,000-node limit
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

func TestGraphQLQueryCommentGating(t *testing.T) {
	issues := (&GithubDownloaderV3{gqlSkipComments: true}).graphQLIssuesQuery()
	assert.NotContains(t, issues, "comments(")
	pulls := (&GithubDownloaderV3{gqlSkipComments: true}).graphQLPullRequestsQuery()
	assert.NotContains(t, pulls, "comments(")
	assert.NotContains(t, pulls, "reviews(")
	assert.NotContains(t, pulls, "reviewRequests(")
	assert.Contains(t, (&GithubDownloaderV3{}).graphQLPullRequestsQuery(), "reviews(first:50)")
}

func TestDoGraphQLPageShrinkSkipsPermanentFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	err = d.doGraphQLPageShrink(t.Context(), `query{viewer{login}}`, map[string]any{}, &struct{}{}, 100, "issues")
	require.Error(t, err)
	assert.NotErrorIs(t, err, errGraphQLTransientExhausted, "an untrusted certificate must not be retried at a smaller page size")
	assert.Zero(t, calls.Load())
}

func TestFetchNodeReactionsSkipsDeletedNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"nodes":[null,{"id":"c2","reactions":{"totalCount":1,"nodes":[{"content":"HEART","user":null}]}}]},`+
			`"errors":[{"type":"NOT_FOUND","path":["nodes",0],"message":"Could not resolve to a node"}]}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)

	got, err := d.fetchNodeReactions(t.Context(), []string{"c1", "c2"})
	require.NoError(t, err)
	require.Len(t, got["c2"], 1)
	assert.Equal(t, "ghost", got["c2"][0].UserName)
	assert.EqualValues(t, 10137, got["c2"][0].UserID)
}

func TestIsGraphQLResourceLimited(t *testing.T) {
	rl := fmt.Errorf("github graphql: %w", graphQLError{Type: "RESOURCE_LIMITS_EXCEEDED", Message: "too expensive"})
	assert.True(t, isGraphQLResourceLimited(rl))
	assert.False(t, isGraphQLResourceLimited(fmt.Errorf("github graphql: %w", graphQLError{Type: "RATE_LIMITED"})))
	assert.False(t, isGraphQLResourceLimited(errors.New("plain")))
}

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

	// comment reactions come from attachCommentReactions, not the content query
	comments := convertGraphQLComments(node.Number, node.Comments.Nodes)
	require.Len(t, comments, 1)
	assert.EqualValues(t, 42, comments[0].IssueIndex)
	assert.EqualValues(t, 900001, comments[0].Index)
	assert.Equal(t, "reporter", comments[0].PosterName)
	assert.Empty(t, comments[0].Reactions, "comment reactions come from the node-id pass, not the content query")
}

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

// a page larger than perPage overflows the DB's bind-parameter limit on insert
func TestGraphQLPageSize(t *testing.T) {
	assert.Equal(t, 41, graphQLPageSize(41, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(500, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(0, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPRPageSize, graphQLPageSize(41, githubGraphQLPRPageSize))
}

func TestGraphQLQueriesSelectNonUserActorIDs(t *testing.T) {
	g := &GithubDownloaderV3{}
	for name, q := range map[string]string{
		"issues":   g.graphQLIssuesQuery(),
		"pulls":    g.graphQLPullRequestsQuery(),
		"timeline": graphQLTimelineBatchQuery,
	} {
		for _, actorType := range []string{"User", "Bot", "Organization", "Mannequin"} {
			assert.Contains(t, q, "... on "+actorType+"{databaseId}", "%s query must select the %s author id", name, actorType)
		}
	}
}

func TestGraphQLQueriesDetectSideConnectionOverflow(t *testing.T) {
	g := &GithubDownloaderV3{}
	issues, pulls := g.graphQLIssuesQuery(), g.graphQLPullRequestsQuery()
	for _, conn := range []string{"labels", "assignees"} {
		assert.Regexp(t, conn+`\(first:\d+\)\{totalCount`, issues, "issue %s must carry totalCount", conn)
		assert.Regexp(t, conn+`\(first:\d+\)\{totalCount`, pulls, "PR %s must carry totalCount", conn)
	}
	assert.Regexp(t, `reviewRequests\(first:\d+\)\{totalCount`, pulls)
}

// a cursor over a mutable sort key such as UPDATED_AT loses rows
func TestGraphQLContentQueriesPageByCreation(t *testing.T) {
	g := &GithubDownloaderV3{}
	for name, q := range map[string]string{"issues": g.graphQLIssuesQuery(), "pulls": g.graphQLPullRequestsQuery()} {
		assert.Contains(t, q, "orderBy:{field:CREATED_AT,direction:ASC}", "%s must page by creation order", name)
		assert.NotContains(t, q, "UPDATED_AT", "%s must not paginate over a mutable sort key", name)
	}
}

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

// the transport retries the 502, doGraphQL the body that died after the headers
func TestDoGraphQLRetriesTransientFailures(t *testing.T) {
	defer test.MockVariableValue(&graphQLTransientRetryBase, time.Millisecond)()
	defer test.MockVariableValue(&retryBaseDelay, time.Millisecond)()

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

func TestDoGraphQLDoesNotReRetryTransportFailures(t *testing.T) {
	defer test.MockVariableValue(&retryBaseDelay, time.Millisecond)()

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

// secondary rate limits come as 403/429, not always with a Retry-After the transport would honour
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
		{"403 secondary limit without Retry-After, quota left", func(w http.ResponseWriter) {
			// the primary window resets in an hour; waiting for it would stall the
			// sweep, a secondary limit only needs the short fallback wait
			w.Header().Set("X-RateLimit-Remaining", "4321")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"You have exceeded a secondary rate limit. Please wait a few minutes before you try again."}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer test.MockVariableValue(&graphQLRateResetFallback, 20*time.Millisecond)()

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

// the framework retries a failed page with the same page number
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

func TestDoGraphQLPageShrink(t *testing.T) {
	defer test.MockVariableValue(&graphQLTransientRetryBase, time.Millisecond)()
	defer test.MockVariableValue(&retryBaseDelay, time.Millisecond)()

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

func TestDoGraphQLPageShrinkOnResourceLimits(t *testing.T) {
	var (
		mu    sync.Mutex
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
			_, _ = io.WriteString(w, `{"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED","message":"Resource limits for this query exceeded."}]}`)
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

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{100, 50, 25}, sizes, "each oversized page is refused once, then halved")
}

// GraphQL points are metered per token
func TestDoGraphQLSwitchesTokenOnRateLimit(t *testing.T) {
	var spentCalls, freshCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer spent" {
			spentCalls.Add(1)
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			_, _ = io.WriteString(w, `{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`)
			return
		}
		freshCalls.Add(1)
		w.Header().Set("X-RateLimit-Remaining", "4000")
		_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "spent,fresh", "o", "r")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var out struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, d.doGraphQL(ctx, "query{}", nil, &out), "must not wait out the spent token's hour")
	assert.True(t, out.OK)
	assert.EqualValues(t, 1, spentCalls.Load())
	assert.EqualValues(t, 1, freshCalls.Load())

	// the next request goes straight to the token that has points
	require.NoError(t, d.doGraphQL(ctx, "query{}", nil, &out))
	assert.EqualValues(t, 1, spentCalls.Load())
	assert.EqualValues(t, 2, freshCalls.Load())
}

func TestPickGraphQLClient(t *testing.T) {
	inAnHour := github.Timestamp{Time: time.Now().Add(time.Hour)}
	d := &GithubDownloaderV3{gqlRates: []*github.Rate{
		{Remaining: 1000, Reset: inAnHour},
		{Remaining: 4000, Reset: inAnHour},
		{Remaining: 5, Reset: inAnHour}, // under the floor
	}}
	require.NoError(t, d.pickGraphQLClient(t.Context()))
	assert.Equal(t, 1, d.curClientIdx, "the token with the most points")

	d.gqlRates[1] = nil
	require.NoError(t, d.pickGraphQLClient(t.Context()))
	assert.Equal(t, 1, d.curClientIdx, "an unknown budget is probed first")

	// every token spent: wait for the earliest reset, then probe that token
	soon := github.Timestamp{Time: time.Now().Add(50 * time.Millisecond)}
	d.gqlRates = []*github.Rate{{Remaining: 0, Reset: inAnHour}, {Remaining: 0, Reset: soon}}
	start := time.Now()
	require.NoError(t, d.pickGraphQLClient(t.Context()))
	assert.Equal(t, 1, d.curClientIdx)
	assert.GreaterOrEqual(t, time.Since(start), 40*time.Millisecond)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestGraphQLSweepsDropCachesPerPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "pullRequests") {
			_, _ = io.WriteString(w, `{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":true,"endCursor":"C"},"nodes":[]}}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":true,"endCursor":"C"},"nodes":[]}}}}`)
	}))
	defer srv.Close()

	d, err := NewGithubDownloaderV3(t.Context(), srv.URL, "", "", "tok", "o", "r")
	require.NoError(t, err)
	d.useGraphQL = true

	d.gqlComments = map[int64][]*base.Comment{1: {{Content: "from page 1"}}}
	_, _, err = d.getIssuesGraphQL(t.Context(), 2, 10)
	require.NoError(t, err)
	assert.NotContains(t, d.gqlComments, int64(1), "issue page 2 must drop page 1's unread comments")

	d.gqlComments = map[int64][]*base.Comment{1: {{Content: "from page 1"}}}
	d.gqlReviews = map[int64][]*base.Review{1: {{Content: "from page 1"}}}
	_, _, err = d.getPullRequestsGraphQL(t.Context(), 2, 10)
	require.NoError(t, err)
	assert.NotContains(t, d.gqlComments, int64(1), "PR page 2 must drop page 1's unread comments")
	assert.NotContains(t, d.gqlReviews, int64(1), "PR page 2 must drop page 1's unread reviews")
}
