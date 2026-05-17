// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/actions/jobparser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

// resetMatrixCache helps individual tests run in isolation against the
// package-level cache map.
func resetMatrixCache() {
	matrixCacheMu.Lock()
	defer matrixCacheMu.Unlock()
	matrixCache = map[matrixCacheKey]matrixCacheEntry{}
}

func buildSingleWorkflow(t *testing.T, name string) *jobparser.SingleWorkflow {
	t.Helper()
	src := []byte("name: " + name + "\non: push\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo " + name + "\n")
	sw := &jobparser.SingleWorkflow{}
	require.NoError(t, yaml.Unmarshal(src, sw))
	return sw
}

func TestMatrixCacheKeyDeterministic(t *testing.T) {
	payload := []byte("hello")
	needs := []string{"a", "b"}
	out := map[string]map[string]string{
		"a": {"k1": "v1", "k2": "v2"},
		"b": {"x": "y"},
	}
	k1 := computeMatrixCacheKey(payload, needs, out)
	// Reorder needs and re-shuffle output keys; key must stay identical.
	k2 := computeMatrixCacheKey(payload, []string{"b", "a"}, map[string]map[string]string{
		"b": {"x": "y"},
		"a": {"k2": "v2", "k1": "v1"},
	})
	assert.Equal(t, k1, k2)
}

func TestMatrixCacheKeyChangesWithInputs(t *testing.T) {
	payload := []byte("hello")
	base := computeMatrixCacheKey(payload, []string{"a"}, map[string]map[string]string{"a": {"k": "v"}})
	differentValue := computeMatrixCacheKey(payload, []string{"a"}, map[string]map[string]string{"a": {"k": "different"}})
	differentPayload := computeMatrixCacheKey([]byte("other"), []string{"a"}, map[string]map[string]string{"a": {"k": "v"}})
	assert.NotEqual(t, base, differentValue)
	assert.NotEqual(t, base, differentPayload)
}

func TestMatrixCachePutGet(t *testing.T) {
	resetMatrixCache()
	key := computeMatrixCacheKey([]byte("p"), []string{"a"}, nil)
	original := []*jobparser.SingleWorkflow{
		buildSingleWorkflow(t, "first"),
		buildSingleWorkflow(t, "second"),
	}
	matrixCachePut(key, original)

	got, ok := matrixCacheGet(key)
	require.True(t, ok, "cache hit expected")
	require.Len(t, got, 2)
	assert.Equal(t, "first", got[0].Name)
	assert.Equal(t, "second", got[1].Name)

	// Cache hit must return fresh values: mutating one must not affect a
	// subsequent get.
	got[0].Name = "tampered"
	got2, _ := matrixCacheGet(key)
	assert.Equal(t, "first", got2[0].Name, "subsequent gets must not see mutations")
}

func TestMatrixCacheExpiry(t *testing.T) {
	resetMatrixCache()
	key := computeMatrixCacheKey([]byte("p"), nil, nil)
	matrixCacheMu.Lock()
	matrixCache[key] = matrixCacheEntry{payloads: [][]byte{[]byte("name: x\non: push\njobs:\n  j:\n    steps:\n      - run: echo\n")}, expiresAt: time.Now().Add(-time.Second)}
	matrixCacheMu.Unlock()
	_, ok := matrixCacheGet(key)
	assert.False(t, ok, "expired entries must miss")
}
