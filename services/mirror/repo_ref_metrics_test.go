// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectRepoRefCountsDisabledWhenMetricsOff(t *testing.T) {
	cfg := refMetricsConfig{Enabled: false, EnabledRefCount: true}

	before := testutil.CollectAndCount(repoRefCount)
	collectRepoRefCountsWith(context.Background(), cfg)
	after := testutil.CollectAndCount(repoRefCount)

	assert.Equal(t, before, after, "gauge series count should not change when Enabled is false")
}

func TestCollectRepoRefCountsDisabledWhenRefCountOff(t *testing.T) {
	cfg := refMetricsConfig{Enabled: true, EnabledRefCount: false}

	before := testutil.CollectAndCount(repoRefCount)
	collectRepoRefCountsWith(context.Background(), cfg)
	after := testutil.CollectAndCount(repoRefCount)

	assert.Equal(t, before, after, "gauge series count should not change when EnabledRefCount is false")
}

func TestCountPackedRefs_Missing(t *testing.T) {
	count := countPackedRefs(filepath.Join(t.TempDir(), "packed-refs"))
	assert.Equal(t, 0, count)
}

func TestCountPackedRefs_HeaderOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packed-refs")
	require.NoError(t, os.WriteFile(path, []byte("# pack-refs with: peeled fully-peeled sorted\n"), 0o644))
	assert.Equal(t, 0, countPackedRefs(path))
}

func TestCountPackedRefs_WithRefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packed-refs")
	content := "# pack-refs with: peeled fully-peeled sorted\n" +
		"abc123def456abc123def456abc123def456abc123 refs/heads/main\n" +
		"^abc123def456abc123def456abc123def456abc124\n" +
		"def456abc123def456abc123def456abc123def456 refs/tags/v1.0\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	assert.Equal(t, 2, countPackedRefs(path))
}

func TestCountLooseRefs_Missing(t *testing.T) {
	count := countLooseRefs(filepath.Join(t.TempDir(), "refs"))
	assert.Equal(t, 0, count)
}

func TestCountLooseRefs_Empty(t *testing.T) {
	assert.Equal(t, 0, countLooseRefs(t.TempDir()))
}

func TestCountLooseRefs_WithFiles(t *testing.T) {
	refsDir := filepath.Join(t.TempDir(), "refs", "heads")
	require.NoError(t, os.MkdirAll(refsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(refsDir, "main"), []byte("abc123\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(refsDir, "feature"), []byte("def456\n"), 0o644))
	assert.Equal(t, 2, countLooseRefs(filepath.Dir(refsDir)))
}
