// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/indexer/issues/internal/tests"
	"code.gitea.io/gitea/modules/json"

	"github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		return err == nil && resp.StatusCode == http.StatusOK
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
