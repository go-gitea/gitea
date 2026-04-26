// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newRealIndexer(t *testing.T) *Indexer {
	t.Helper()
	url := "http://elasticsearch:9200"
	if os.Getenv("CI") == "" {
		url = os.Getenv("TEST_ELASTICSEARCH_URL")
		if url == "" {
			t.Skip("TEST_ELASTICSEARCH_URL not set and not running in CI")
		}
	}
	indexName := "gitea_test_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	ix := NewIndexer(url, indexName, 1, `{"mappings":{"properties":{"x":{"type":"keyword"}}}}`)
	_, err := ix.Init(t.Context())
	require.NoError(t, err)
	t.Cleanup(ix.Close)
	return ix
}

func TestPing(t *testing.T) {
	ix := newRealIndexer(t)
	require.NoError(t, ix.Ping(t.Context()))
}

func TestDeleteSwallows404(t *testing.T) {
	ix := newRealIndexer(t)
	require.NoError(t, ix.Delete(t.Context(), "missing-id"))
}

func TestBulkAcceptsDelete404(t *testing.T) {
	ix := newRealIndexer(t)
	require.NoError(t, ix.Bulk(t.Context(), []BulkOp{DeleteOp("missing-id")}))
}
