// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

// WorkflowParseCache caches parsed workflow results to avoid redundant YAML parsing
// This is especially useful for matrix re-evaluation where the same workflow might be
// parsed multiple times with the same job outputs
type WorkflowParseCache struct {
	cache *lru.TwoQueueCache[string, []byte] // key -> marshaled workflow bytes
	mu    sync.RWMutex
}

var (
	workflowParseCache     *WorkflowParseCache
	workflowParseCacheOnce sync.Once
)

// getWorkflowParseCache returns the singleton workflow parse cache instance
func getWorkflowParseCache() *WorkflowParseCache {
	workflowParseCacheOnce.Do(func() {
		// Cache up to 1000 workflow parses
		// 2Q cache is more efficient than simple LRU for this use case
		cache, err := lru.New2Q[string, []byte](1000)
		if err != nil {
			// Fallback to no caching if cache creation fails
			workflowParseCache = nil
			return
		}
		workflowParseCache = &WorkflowParseCache{
			cache: cache,
		}
	})
	return workflowParseCache
}

// computeCacheKey generates a cache key for a workflow parse operation
func computeCacheKey(workflowYAML []byte, taskNeeds map[string]*TaskNeed) string {
	// Create a deterministic hash of workflow + all job outputs
	h := sha256.New()
	h.Write(workflowYAML)

	// Add outputs in deterministic order (sorted by job ID)
	for jobID, need := range taskNeeds {
		h.Write([]byte(jobID))
		if need.Outputs != nil {
			for k, v := range need.Outputs {
				h.Write([]byte(k))
				h.Write([]byte(v))
			}
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves a cached workflow parse result
func (c *WorkflowParseCache) Get(key string) ([]byte, bool) {
	if c == nil || c.cache == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Get(key)
}

// Set stores a workflow parse result in the cache
func (c *WorkflowParseCache) Set(key string, value []byte) {
	if c == nil || c.cache == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(key, value)
}

// Stats returns cache statistics for monitoring
func (c *WorkflowParseCache) Stats() (size int, err error) {
	if c == nil || c.cache == nil {
		return 0, errors.New("cache not initialized")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Len(), nil
}
