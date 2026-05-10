// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/require"
)

func newRealIndexer(t *testing.T) *Indexer {
	t.Helper()
	esURL := util.IfZero(os.Getenv("TEST_ELASTICSEARCH_URL"), "http://elasticsearch:9200")
	resp, err := http.Get(esURL)
	if err != nil && test.AllowSkipExternalService() {
		t.Skip("elastic search server not found, skipped")
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	indexName := "gitea_test_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	ix := NewIndexer(esURL, indexName, 1, `{"mappings":{"properties":{"x":{"type":"keyword"}}}}`)
	_, err = ix.Init(t.Context())
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
