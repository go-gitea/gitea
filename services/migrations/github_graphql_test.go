// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/test"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGithubTestDownloader(t *testing.T, handler http.HandlerFunc, spareTokens ...string) *GithubGraphQLDownloader {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-RateLimit-Remaining", "4000")
		handler(writer, request)
	}))
	t.Cleanup(server.Close)
	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", strings.Join(append([]string{"tok"}, spareTokens...), ","), "o", "r")
	require.NoError(t, err)
	return newGithubGraphQLDownloader(downloader, false)
}

func TestGraphQLEndpoint(t *testing.T) {
	for baseURL, endpoint := range map[string]string{
		"https://github.com":       "https://api.github.com/graphql",
		"https://api.github.com":   "https://api.github.com/graphql",
		"https://ghe.example.com":  "https://ghe.example.com/api/graphql",
		"https://ghe.example.com/": "https://ghe.example.com/api/graphql",
	} {
		downloader, err := NewGithubDownloaderV3(t.Context(), baseURL, "", "", "tok", "o", "r")
		require.NoError(t, err)
		assert.Equal(t, endpoint, newGithubGraphQLDownloader(downloader, false).graphQLEndpoint(), baseURL)
	}
}

func TestGraphQLRateResetWait(t *testing.T) {
	for _, testcase := range []struct {
		name    string
		headers map[string]string
		want    time.Duration
	}{
		{"retry after takes precedence", map[string]string{"Retry-After": "30", "X-RateLimit-Reset": "9999999999"}, 30 * time.Second},
		{"no headers", nil, graphQLRateResetFallback},
		{"past primary reset", map[string]string{"X-RateLimit-Reset": "1"}, graphQLRateResetFallback},
		{"secondary reset is not primary reset", map[string]string{"X-RateLimit-Remaining": "4321", "X-RateLimit-Reset": "9999999999"}, graphQLRateResetFallback},
		{"primary reset is capped", map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "9999999999"}, graphQLRateResetCap},
		{"retry after is capped", map[string]string{"Retry-After": "999999"}, graphQLRateResetCap},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			header := http.Header{}
			for name, value := range testcase.headers {
				header.Set(name, value)
			}
			assert.Equal(t, testcase.want, graphQLRateResetWait(header))
		})
	}
}

func TestDoGraphQLRetriesTransientFailures(t *testing.T) {
	defer test.MockVariableValue(&graphQLTransientRetryBase, time.Duration(0))()
	for _, testcase := range []struct {
		name     string
		failure  string
		failures int32
	}{
		{"bad gateway then success", "502", 1},
		{"truncated body then success", "body", 1},
		{"connection closed then success", "connection", 1},
		{"persistent bad gateway", "502", maxGraphQLTransientRetries + 1},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			var calls atomic.Int32
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, _request *http.Request) {
				if calls.Add(1) > testcase.failures {
					_, _ = io.WriteString(writer, `{"data":{"ok":true}}`)
					return
				}
				switch testcase.failure {
				case "502":
					writer.WriteHeader(http.StatusBadGateway)
				case "body":
					_, _ = io.WriteString(writer, `{"data":{"ok":`)
				case "connection":
					connection, _, err := http.NewResponseController(writer).Hijack()
					if assert.NoError(t, err) {
						_ = connection.Close()
					}
				}
			})
			var response struct {
				OK bool `json:"ok"`
			}
			err := downloader.doGraphQL(t.Context(), "query{}", nil, &response)
			if testcase.failures > maxGraphQLTransientRetries {
				require.ErrorIs(t, err, errGraphQLTransientExhausted)
				assert.Equal(t, int32(maxGraphQLTransientRetries+1), calls.Load())
			} else {
				require.NoError(t, err)
				assert.True(t, response.OK)
				assert.Equal(t, testcase.failures+1, calls.Load())
			}
		})
	}
}

func TestDoGraphQLWaitsOutSecondaryRateLimit(t *testing.T) {
	defer test.MockVariableValue(&graphQLRateResetFallback, time.Millisecond)()
	for _, testcase := range []struct {
		name   string
		status int
		body   string
	}{
		{"GraphQL rate limit", http.StatusOK, `{"errors":[{"type":"RATE_LIMITED"}]}`},
		{"429 without retry after", http.StatusTooManyRequests, ""},
		{"403 spent primary quota", http.StatusForbidden, ""},
		{"403 secondary limit with quota left", http.StatusForbidden, `{"message":"You have exceeded a secondary rate limit."}`},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			tokens := make(chan string, 3)
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
				tokens <- request.Header.Get("Authorization")
				if request.Header.Get("Authorization") == "Bearer tok" {
					writer.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
					if testcase.body == "" {
						writer.Header().Set("X-RateLimit-Remaining", "0")
					}
					writer.WriteHeader(testcase.status)
					_, _ = io.WriteString(writer, testcase.body)
					return
				}
				_, _ = io.WriteString(writer, `{"data":{"ok":true}}`)
			}, "spare")
			var response struct {
				OK bool `json:"ok"`
			}
			require.NoError(t, downloader.doGraphQL(t.Context(), "query{}", nil, &response))
			assert.True(t, response.OK)
			assert.Equal(t, []string{"Bearer tok", "Bearer spare"}, []string{<-tokens, <-tokens})
			assert.Zero(t, downloader.gqlRates[0].Remaining)
		})
	}
}

func TestDoGraphQLRateRetryBudgetPerToken(t *testing.T) {
	defer test.MockVariableValue(&graphQLRateResetFallback, time.Duration(0))()
	var calls atomic.Int32
	downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, _request *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(writer, `{"errors":[{"type":"RATE_LIMITED"}]}`)
	}, "spare")
	require.Error(t, downloader.doGraphQL(t.Context(), "query{}", nil, &struct{}{}))
	assert.Equal(t, int32(maxGraphQLRateRetries*len(downloader.clients)+1), calls.Load())
}

func TestDoGraphQLFatalStatusQuotesBody(t *testing.T) {
	var calls atomic.Int32
	downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, _request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(writer, `{"message":"Resource not accessible by integration"}`)
	})
	assert.ErrorContains(t, downloader.doGraphQL(t.Context(), "query{}", nil, &struct{}{}), "Resource not accessible by integration")
	assert.EqualValues(t, 1, calls.Load())
}

func TestPickGraphQLClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reset := github.Timestamp{Time: time.Now().Add(2 * graphQLRateResetCap)}
		downloader := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{}, gqlRates: []*github.Rate{
			{Remaining: 1000, Reset: reset},
			{Remaining: 4000, Reset: reset},
			{Remaining: 5, Reset: reset},
		}}
		require.NoError(t, downloader.pickGraphQLClient(t.Context()))
		assert.Equal(t, 1, downloader.curClientIdx)
		downloader.gqlRates[2] = nil
		require.NoError(t, downloader.pickGraphQLClient(t.Context()))
		assert.Equal(t, 2, downloader.curClientIdx)
		downloader.gqlRates = []*github.Rate{{Reset: github.Timestamp{Time: reset.Add(time.Hour)}}, {Reset: reset}}
		started := time.Now()
		require.NoError(t, downloader.pickGraphQLClient(t.Context()))
		assert.Equal(t, graphQLRateResetCap, time.Since(started))
		assert.Equal(t, 1, downloader.curClientIdx)
		assert.Nil(t, downloader.gqlRates[1])
	})
}

func TestIsTransientTransportError(t *testing.T) {
	wrapped := func(err error) error {
		return &url.Error{Op: "Post", URL: "https://example.com", Err: &net.OpError{Op: "dial", Err: err}}
	}
	for _, testcase := range []struct {
		name      string
		err       error
		transient bool
	}{
		{"blocked by policy", fmt.Errorf("blocked: %w", policy.ErrDenied), false},
		{"host does not resolve", &net.DNSError{IsNotFound: true}, false},
		{"TLS verification failed", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, false},
		{"DNS server failure", &net.DNSError{IsTemporary: true}, true},
		{"connection reset", errors.New("connection reset by peer"), true},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			assert.Equal(t, testcase.transient, isTransientTransportError(wrapped(testcase.err)))
		})
	}
}

func TestGraphQLQueryGating(t *testing.T) {
	for _, skipComments := range []bool{false, true} {
		for _, skipReactions := range []bool{false, true} {
			downloader := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{SkipReactions: skipReactions}, skipComments: skipComments}
			for name, query := range map[string]string{"issues": downloader.graphQLIssuesQuery(), "pullRequests": downloader.graphQLPullRequestsQuery()} {
				t.Run(fmt.Sprintf("%s/comments=%t/reactions=%t", name, !skipComments, !skipReactions), func(t *testing.T) {
					assert.Equal(t, !skipComments, strings.Contains(query, "comments("))
					assert.Equal(t, !skipComments && name == "pullRequests", strings.Contains(query, "reviews("))
					assert.Equal(t, !skipComments && name == "pullRequests", strings.Contains(query, "reviewRequests("))
					assert.Equal(t, !skipReactions, strings.Contains(query, "reactions("))
					assert.LessOrEqual(t, strings.Count(query, "reactions("), 1)
					assert.Contains(t, query, "id number title")
					assert.Contains(t, query, "orderBy:{field:CREATED_AT,direction:ASC}")
					assert.Contains(t, query, "labels(first:30){"+gqlPageInfoFields)
					assert.Contains(t, query, "... on Bot{databaseId}")
				})
			}
		}
	}
}

func TestGraphQLPageSize(t *testing.T) {
	assert.Equal(t, 41, graphQLPageSize(41, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(500, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPageSize, graphQLPageSize(0, githubGraphQLPageSize))
	assert.Equal(t, githubGraphQLPRPageSize, graphQLPageSize(41, githubGraphQLPRPageSize))
}

func TestDoGraphQLPageShrink(t *testing.T) {
	defer test.MockVariableValue(&graphQLTransientRetryBase, time.Duration(0))()
	for _, failure := range []string{"502", "RESOURCE_LIMITS_EXCEEDED"} {
		t.Run(failure, func(t *testing.T) {
			sizes := make(chan int, 16)
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
				var payload struct {
					Variables struct {
						First  int    `json:"first"`
						Cursor string `json:"cursor"`
					} `json:"variables"`
				}
				assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
				sizes <- payload.Variables.First
				assert.Equal(t, "CURSOR", payload.Variables.Cursor)
				if payload.Variables.First > 25 {
					if failure == "502" {
						writer.WriteHeader(http.StatusBadGateway)
					} else {
						_, _ = io.WriteString(writer, `{"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED"}]}`)
					}
					return
				}
				_, _ = io.WriteString(writer, `{"data":{}}`)
			})
			require.NoError(t, downloader.doGraphQLPage(t.Context(), downloader.graphQLIssuesQuery(), "CURSOR", 100, &gqlIssuesResponse{}))
			close(sizes)
			var got, want []int
			for size := range sizes {
				got = append(got, size)
			}
			for _, size := range []int{100, 50, 25} {
				repeats := 1
				if failure == "502" && size > 25 {
					repeats += maxGraphQLTransientRetries
				}
				for range repeats {
					want = append(want, size)
				}
			}
			assert.Equal(t, want, got)
		})
	}
}

func TestFetchNodeReactionsBisectsAndSkipsDeletedNodes(t *testing.T) {
	defer test.MockVariableValue(&graphQLTransientRetryBase, time.Duration(0))()
	for _, failure := range []string{"502", "RESOURCE_LIMITS_EXCEEDED"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				var payload struct {
					Variables struct {
						IDs []string `json:"ids"`
					} `json:"variables"`
				}
				assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
				if len(payload.Variables.IDs) > 1 {
					if failure == "502" {
						writer.WriteHeader(http.StatusBadGateway)
					} else {
						_, _ = io.WriteString(writer, `{"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED"}]}`)
					}
					return
				}
				if payload.Variables.IDs[0] == "deleted" {
					_, _ = io.WriteString(writer, `{"data":{"nodes":[null]},"errors":[{"type":"NOT_FOUND","path":["nodes",0]}]}`)
				} else {
					_, _ = io.WriteString(writer, `{"data":{"nodes":[{"id":"present","reactions":{"nodes":[{"content":"HEART","user":null}]}}]}}`)
				}
			})
			got, err := downloader.fetchNodeReactions(t.Context(), []string{"deleted", "present"})
			require.NoError(t, err)
			assert.Equal(t, map[string][]*base.Reaction{"present": {{UserName: "ghost", Content: "heart"}}}, got)
			if failure == "502" {
				assert.EqualValues(t, maxGraphQLTransientRetries+3, calls.Load())
			} else {
				assert.EqualValues(t, 3, calls.Load())
			}
		})
	}
}

func TestLabelsSweepContinuesFromEndCursor(t *testing.T) {
	downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
		var payload graphQLRequest
		assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		assert.Equal(t, "I_node", payload.Variables["id"])
		assert.Equal(t, "LABEL1", payload.Variables["cursor"])
		_, _ = io.WriteString(writer, `{"data":{"node":{"conn":{"nodes":[{"name":"second"}]}}}}`)
	})
	labels, err := allNodes(t.Context(), downloader, graphQLLabelsSweepQuery, "I_node", gqlConn[gqlLabel]{
		PageInfo: gqlPageInfo{HasNextPage: true, EndCursor: "LABEL1"},
		Nodes:    []gqlLabel{{Name: "first"}},
	})
	require.NoError(t, err)
	assert.Equal(t, []gqlLabel{{Name: "first"}, {Name: "second"}}, labels)
}

func TestGraphQLIssuesResponseParsing(t *testing.T) {
	var response gqlIssuesResponse
	require.NoError(t, json.Unmarshal([]byte(`{"repository":{"issues":{"nodes":[{
		"id":"I_42","number":42,"title":"a bug","body":"it broke","state":"CLOSED","locked":true,
		"createdAt":"2024-01-02T03:04:05Z","updatedAt":"2024-02-03T04:05:06Z","closedAt":"2024-02-03T04:05:06Z",
		"author":{"__typename":"Bot","login":"reporter","databaseId":11},"milestone":{"title":"v1.0"},
		"labels":{"nodes":[{"name":"bug","color":"d73a4a","description":"defect"}]},
		"assignees":{"nodes":[{"login":"maintainer"}]},
		"reactions":{"nodes":[{"content":"THUMBS_UP","user":{"login":"alice","databaseId":22}}]},
		"comments":{"nodes":[{"id":"IC_1","databaseId":900,"body":"same here","author":null}]}
	}]}}}`), &response))
	downloader := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{}}
	issue, err := downloader.convertGraphQLIssue(t.Context(), &response.Repository.Issues.Nodes[0], map[string]*gqlPrefetched{})
	require.NoError(t, err)
	updated := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	assert.Equal(t, &base.Issue{
		Number: 42, Title: "a bug", Content: "it broke", State: "closed", IsLocked: true,
		PosterName: "reporter[bot]", PosterID: 11, Milestone: "v1.0", ForeignIndex: 42,
		Created: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), Updated: updated, Closed: &updated,
		Labels: []*base.Label{{Name: "bug", Color: "d73a4a", Description: "defect"}}, Assignees: []string{"maintainer"},
		Reactions: []*base.Reaction{{Content: "+1", UserName: "alice", UserID: 22}}, Context: issue.Context,
	}, issue)
	comments, _, err := downloader.GetComments(t.Context(), issue)
	require.NoError(t, err)
	assert.Equal(t, []*base.Comment{{IssueIndex: 42, Index: 900, Content: "same here", PosterName: "ghost"}}, comments)
	assert.False(t, downloader.SupportGetRepoComments())
}

func TestGraphQLKeepsCursorOnPageError(t *testing.T) {
	for _, connection := range []string{"issues", "pullRequests"} {
		for _, phase := range []string{"timelineItems", "Reactable"} {
			t.Run(connection+"/"+phase, func(t *testing.T) {
				var failPage atomic.Bool
				failPage.Store(true)
				downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
					var payload graphQLRequest
					assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
					if strings.Contains(payload.Query, "repository(owner:") {
						assert.Equal(t, "PAGE1", payload.Variables["cursor"])
						_, _ = fmt.Fprintf(writer, `{"data":{"repository":{"%s":{"pageInfo":{"endCursor":"PAGE2"},"nodes":[{"id":"I_1","number":1,"comments":{"nodes":[{"id":"IC_1"}]}}]}}}}`, connection)
					} else if failPage.Load() && strings.Contains(payload.Query, phase) {
						writer.WriteHeader(http.StatusBadRequest)
					} else {
						_, _ = io.WriteString(writer, `{"data":{"nodes":[]}}`)
					}
				})
				cursor := &downloader.issuesCursor
				if connection == "pullRequests" {
					cursor = &downloader.prCursor
				}
				*cursor = "PAGE1"
				fetch := func() error {
					if connection == "issues" {
						_, _, err := downloader.getIssuesGraphQL(t.Context(), 2, 10)
						return err
					} else {
						_, _, err := downloader.getPullRequestsGraphQL(t.Context(), 2, 10)
						return err
					}
				}
				require.Error(t, fetch())
				assert.Equal(t, "PAGE1", *cursor)
				failPage.Store(false)
				require.NoError(t, fetch())
				assert.Equal(t, "PAGE2", *cursor)
			})
		}
	}
}
