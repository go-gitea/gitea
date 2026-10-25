// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type githubTestTransport func(*http.Request) (*http.Response, error)

func (f githubTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGithubCommentRateLimitRecovery(t *testing.T) {
	for _, mode := range []string{"past reset", "future reset", "secondary", "forbidden", "cancel", "exhaust"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				lists, reactions := 0, 0
				started := time.Now()
				transport := &http.Transport{}
				transport.RegisterProtocol("https", githubTestTransport(func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, "Bearer token", req.Header.Get("Authorization"))
					resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req}
					resp.Header.Set("Content-Type", "application/json")
					resp.Header.Set("X-RateLimit-Remaining", "100")
					body := `[{"id":77,"issue_url":"https://api.github.com/repos/o/r/issues/473","body":"comment","reactions":{"total_count":1}}]`
					if strings.HasSuffix(req.URL.Path, "/reactions") {
						reactions++
						body = `[{"content":"+1","user":{"id":1,"login":"author"}}]`
						if reactions == 1 || mode == "exhaust" {
							resp.StatusCode = http.StatusForbidden
							resp.Header.Set("X-RateLimit-Remaining", "0")
							reset := started.Add(-time.Second)
							if mode == "future reset" {
								reset = started.Add(time.Hour)
							}
							resp.Header.Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
							body = `{"message":"API rate limit exceeded"}`
							if mode == "secondary" {
								resp.Header.Set("X-RateLimit-Remaining", "100")
								resp.Header.Set("Retry-After", "60")
								body = `{"message":"secondary rate limit","documentation_url":"https://docs.github.com/#secondary-rate-limits"}`
							}
							if mode == "forbidden" {
								resp.Header.Set("X-RateLimit-Remaining", "100")
								body = `{"message":"Resource not accessible"}`
							}
							if mode == "cancel" {
								time.AfterFunc(100*time.Millisecond, cancel)
							}
						}
					} else {
						lists++
					}
					resp.Body = io.NopCloser(strings.NewReader(body))
					return resp, nil
				}))
				defer test.MockVariableValue(&getMigrationTransport, func() *http.Transport { return transport })()
				downloader, err := NewGithubDownloaderV3(ctx, "https://github.com", "", "", "token", "o", "r")
				require.NoError(t, err)
				comments, end, err := downloader.GetAllComments(ctx, 1, 100)
				assert.Equal(t, 1, lists)
				switch mode {
				case "forbidden":
					require.ErrorContains(t, err, "Resource not accessible")
					assert.Equal(t, 1, reactions)
				case "cancel":
					require.ErrorIs(t, err, context.Canceled)
					assert.Equal(t, 1, reactions)
				case "exhaust":
					require.ErrorContains(t, err, "API rate limit exceeded")
					assert.Equal(t, githubRateLimitAttempts, reactions)
				default:
					require.NoError(t, err)
					require.Len(t, comments, 1)
					assert.Equal(t, int64(473), comments[0].IssueIndex)
					assert.Len(t, comments[0].Reactions, 1)
					assert.True(t, end)
					assert.Equal(t, 2, reactions)
					minimum := time.Second
					if mode == "future reset" {
						minimum = time.Hour + time.Second
					} else if mode == "secondary" {
						minimum = time.Minute
					}
					assert.GreaterOrEqual(t, time.Since(started), minimum)
				}
			})
		})
	}
}
