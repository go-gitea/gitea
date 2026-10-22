// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/modules/indexer/issues/internal"
	"gitea.dev/modules/indexer/issues/internal/tests"
	"gitea.dev/modules/json"
	"gitea.dev/modules/test"

	"github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexTaskCompletion(t *testing.T) {
	for _, status := range []string{"succeeded", "failed", "canceled", "unavailable", "cancel-context"} {
		t.Run(status, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var polls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPost:
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprint(w, `{"taskUid":42,"status":"enqueued"}`)
				case r.URL.Path == "/tasks/42":
					if polls.Add(1) == 1 {
						fmt.Fprint(w, `{"uid":42,"status":"processing"}`)
						return
					}
					switch status {
					case "unavailable":
						w.WriteHeader(http.StatusServiceUnavailable)
					case "cancel-context":
						cancel()
						fmt.Fprint(w, `{"uid":42,"status":"processing"}`)
					default:
						fmt.Fprintf(w, `{"uid":42,"status":%q,"error":{"message":"task rejected"}}`, status)
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			indexer := NewIndexer(server.URL, "", "test")
			indexer.inner.Client = meilisearch.New(server.URL, meilisearch.WithCustomRetries(nil, 0))
			err := indexer.Index(ctx, &internal.IndexerData{ID: 1})
			if status == "succeeded" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if status == "cancel-context" {
					assert.ErrorIs(t, ctx.Err(), context.Canceled)
					assert.ErrorContains(t, err, context.Canceled.Error())
				}
			}
			assert.GreaterOrEqual(t, polls.Load(), int32(2))
		})
	}
}

func TestMeilisearchIndexer(t *testing.T) {
	// The meilisearch instance started by pull-db-tests.yml > test-unit > services > meilisearch
	url := test.ExternalServiceHTTP(t, "TEST_MEILISEARCH_URL", "http://meilisearch:7700")
	key := os.Getenv("TEST_MEILISEARCH_KEY")

	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, time.Minute, time.Second, "Expected meilisearch to be up")

	indexer := NewIndexer(url, key, fmt.Sprintf("test_meilisearch_indexer_%d", time.Now().Unix()))
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}

func TestConvertHits(t *testing.T) {
	convert := func(d any) []byte {
		b, _ := json.Marshal(d)
		return b
	}

	_, err := convertHits(&meilisearch.SearchResponse{
		Hits: []meilisearch.Hit{
			{
				"aa": convert(1),
				"bb": convert(2),
				"cc": convert(3),
				"dd": convert(4),
			},
		},
	})
	assert.ErrorIs(t, err, ErrMalformedResponse)

	validResponse := &meilisearch.SearchResponse{
		Hits: []meilisearch.Hit{
			{
				"id":       convert(float64(11)),
				"title":    convert("a title"),
				"content":  convert("issue body with no match"),
				"comments": convert([]any{"hey whats up?", "I'm currently bowling", "nice"}),
			},
			{
				"id":       convert(float64(22)),
				"title":    convert("Bowling as title"),
				"content":  convert(""),
				"comments": convert([]any{}),
			},
			{
				"id":       convert(float64(33)),
				"title":    convert("Bowl-ing as fuzzy match"),
				"content":  convert(""),
				"comments": convert([]any{}),
			},
		},
	}
	hits, err := convertHits(validResponse)
	assert.NoError(t, err)
	assert.Equal(t, []internal.Match{{ID: 11}, {ID: 22}, {ID: 33}}, hits)
}

func TestDoubleQuoteKeyword(t *testing.T) {
	assert.Empty(t, doubleQuoteKeyword(""))
	assert.Equal(t, `"a" "b" "c"`, doubleQuoteKeyword("a b c"))
	assert.Equal(t, `"a" "d" "g"`, doubleQuoteKeyword("a  d g"))
	assert.Equal(t, `"a" "d" "g"`, doubleQuoteKeyword("a  d g"))
	assert.Equal(t, `"a" "d" "g"`, doubleQuoteKeyword(`a  "" "d" """g`))
}
