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

	"github.com/stretchr/testify/require"
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

	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		return err == nil && resp.StatusCode == http.StatusOK
	}, time.Minute, time.Second, "Expected elasticsearch to be up")

	indexer := NewIndexer(url, fmt.Sprintf("test_elasticsearch_indexer_%d", time.Now().Unix()))
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}
