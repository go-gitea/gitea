// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/indexer/issues/internal/tests"
)

func TestElasticsearchIndexer(t *testing.T) {
	// The elasticsearch instance started by pull-db-tests.yml > test-unit > services > elasticsearch
	url := "http://elastic:changeme@elasticsearch:9200"

	if os.Getenv("CI") == "" {
		// Make it possible to run tests against a local elasticsearch instance
		url = os.Getenv("TEST_ELASTICSEARCH_URL")
		if url == "" {
			t.Skip("TEST_ELASTICSEARCH_URL not set and not running in CI")
			return
		}
	}

	ok := false
	for i := 0; i < 60; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			ok = true
			break
		}
		t.Logf("Waiting for elasticsearch to be up: %v", err)
		time.Sleep(time.Second)
	}
	if !ok {
		t.Fatalf("Failed to wait for elasticsearch to be up")
		return
	}

	indexer := NewIndexer(url, fmt.Sprintf("test_elasticsearch_indexer_%d", time.Now().Unix()))
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}
