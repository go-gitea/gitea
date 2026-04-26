// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// newRealIndexer connects to the ES instance pointed at by TEST_INDEXER_CODE_ES_URL,
// creating a fresh per-test index. Tests are skipped when the env var is unset.
func newRealIndexer(t *testing.T) *Indexer {
	t.Helper()
	u := os.Getenv("TEST_INDEXER_CODE_ES_URL")
	if u == "" {
		t.Skip("TEST_INDEXER_CODE_ES_URL not set")
	}
	indexName := "gitea_test_" + strings.ToLower(t.Name())
	ix := NewIndexer(u, indexName, 1, `{"mappings":{"properties":{"x":{"type":"keyword"}}}}`)
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
