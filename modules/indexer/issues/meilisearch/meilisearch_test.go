// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/indexer/issues/internal/tests"
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
