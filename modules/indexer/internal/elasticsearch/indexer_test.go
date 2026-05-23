// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/require"
)

func newRealIndexer(t *testing.T) *Indexer {
	t.Helper()
	esURL := test.ExternalServiceHTTP(t, "TEST_ELASTICSEARCH_URL", "http://elasticsearch:9200")
	indexName := "gitea_test_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	ix := NewIndexer(esURL, indexName, 1, `{"mappings":{"properties":{"x":{"type":"keyword"}}}}`)
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
