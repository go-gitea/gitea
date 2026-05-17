// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/sha256"
	"sort"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/actions/jobparser"

	"go.yaml.in/yaml/v4"
)

// Deferred-matrix expansion cache.
//
// The expansion is content-deterministic: identical (placeholder payload,
// needs list, needs outputs) always produces identical N SingleWorkflows.
// This cache lets the emitter skip re-parsing + re-evaluating when the
// same placeholder is processed more than once (e.g. a queue retry, or
// two runs whose placeholders + upstream outputs happen to match).
//
// Entries are stored as marshalled YAML so cache hits return fresh
// SingleWorkflow values the caller is free to mutate (the expansion path
// EraseNeeds+SetJob+Marshal each child). TTL keeps the map bounded; the
// expected working set is tiny (one entry per concurrent expansion).

const matrixCacheTTL = 30 * time.Second

type matrixCacheKey [sha256.Size]byte

type matrixCacheEntry struct {
	payloads  [][]byte
	expiresAt time.Time
}

var (
	matrixCacheMu sync.Mutex
	matrixCache   = map[matrixCacheKey]matrixCacheEntry{}
)

// computeMatrixCacheKey hashes the inputs that determine the expansion result.
// Map iteration is sorted so map-key order doesn't perturb the hash.
func computeMatrixCacheKey(payload []byte, needs []string, outputs map[string]map[string]string) matrixCacheKey {
	h := sha256.New()
	h.Write(payload)
	h.Write([]byte{0})

	sortedNeeds := append([]string(nil), needs...)
	sort.Strings(sortedNeeds)
	for _, need := range sortedNeeds {
		h.Write([]byte(need))
		h.Write([]byte{0})
		needOutputs := outputs[need]
		keys := make([]string, 0, len(needOutputs))
		for k := range needOutputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte{0})
			h.Write([]byte(needOutputs[k]))
			h.Write([]byte{0})
		}
		h.Write([]byte{0})
	}

	var key matrixCacheKey
	copy(key[:], h.Sum(nil))
	return key
}

func matrixCacheGet(key matrixCacheKey) ([]*jobparser.SingleWorkflow, bool) {
	matrixCacheMu.Lock()
	defer matrixCacheMu.Unlock()
	entry, ok := matrixCache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(matrixCache, key)
		return nil, false
	}
	out := make([]*jobparser.SingleWorkflow, len(entry.payloads))
	for i, p := range entry.payloads {
		sw := &jobparser.SingleWorkflow{}
		if err := yaml.Unmarshal(p, sw); err != nil {
			// A cache entry that fails to unmarshal is unusable; drop it and
			// behave like a miss so the caller will recompute.
			delete(matrixCache, key)
			return nil, false
		}
		out[i] = sw
	}
	return out, true
}

func matrixCachePut(key matrixCacheKey, expanded []*jobparser.SingleWorkflow) {
	payloads := make([][]byte, 0, len(expanded))
	for _, sw := range expanded {
		b, err := sw.Marshal()
		if err != nil {
			// Don't cache partial results — caller will recompute.
			return
		}
		payloads = append(payloads, b)
	}
	matrixCacheMu.Lock()
	defer matrixCacheMu.Unlock()
	matrixCache[key] = matrixCacheEntry{
		payloads:  payloads,
		expiresAt: time.Now().Add(matrixCacheTTL),
	}
	// Opportunistically evict expired entries to keep the map bounded.
	now := time.Now()
	for k, v := range matrixCache {
		if now.After(v.expiresAt) {
			delete(matrixCache, k)
		}
	}
}
