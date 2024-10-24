// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/indexer/conversations/internal/tests"

	"github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/assert"
)

func TestMeilisearchIndexer(t *testing.T) {
	// The meilisearch instance started by pull-db-tests.yml > test-unit > services > meilisearch
	url := "http://meilisearch:7700"
	key := "" // auth has been disabled in test environment

	if os.Getenv("CI") == "" {
		// Make it possible to run tests against a local meilisearch instance
		url = os.Getenv("TEST_MEILISEARCH_URL")
		if url == "" {
			t.Skip("TEST_MEILISEARCH_URL not set and not running in CI")
			return
		}
		key = os.Getenv("TEST_MEILISEARCH_KEY")
	}

	ok := false
	for i := 0; i < 60; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			ok = true
			break
		}
		t.Logf("Waiting for meilisearch to be up: %v", err)
		time.Sleep(time.Second)
	}
	if !ok {
		t.Fatalf("Failed to wait for meilisearch to be up")
		return
	}

	indexer := NewIndexer(url, key, fmt.Sprintf("test_meilisearch_indexer_%d", time.Now().Unix()))
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}

func TestConvertHits(t *testing.T) {
	_, err := convertHits(&meilisearch.SearchResponse{
		Hits: []any{"aa", "bb", "cc", "dd"},
	})
	assert.ErrorIs(t, err, ErrMalformedResponse)

	validResponse := &meilisearch.SearchResponse{
		Hits: []any{
			map[string]any{
				"id":       float64(11),
				"title":    "a title",
				"content":  "conversation body with no match",
				"comments": []any{"hey whats up?", "I'm currently bowling", "nice"},
			},
			map[string]any{
				"id":       float64(22),
				"title":    "Bowling as title",
				"content":  "",
				"comments": []any{},
			},
			map[string]any{
				"id":       float64(33),
				"title":    "Bowl-ing as fuzzy match",
				"content":  "",
				"comments": []any{},
			},
		},
	}
	hits, err := convertHits(validResponse)
	assert.NoError(t, err)
	assert.EqualValues(t, []internal.Match{{ID: 11}, {ID: 22}, {ID: 33}}, hits)
}

func TestDoubleQuoteKeyword(t *testing.T) {
	assert.EqualValues(t, "", doubleQuoteKeyword(""))
	assert.EqualValues(t, `"a" "b" "c"`, doubleQuoteKeyword("a b c"))
	assert.EqualValues(t, `"a" "d" "g"`, doubleQuoteKeyword("a  d g"))
	assert.EqualValues(t, `"a" "d" "g"`, doubleQuoteKeyword("a  d g"))
	assert.EqualValues(t, `"a" "d" "g"`, doubleQuoteKeyword(`a  "" "d" """g`))
}
