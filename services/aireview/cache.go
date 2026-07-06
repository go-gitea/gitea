// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"sync"
)

// commitCache tracks which PR commits have already been reviewed.
// It resets on server restart, which is fine — re-review after restart
// is better than missing a review.
type commitCache struct {
	mu       sync.RWMutex
	reviewed map[int64]string // PRID → head commit SHA (last reviewed)
}

var reviewCache = &commitCache{
	reviewed: make(map[int64]string),
}

// IsAlreadyReviewed returns true if the given PR at the given commit
// has already been reviewed.
func (c *commitCache) IsAlreadyReviewed(prID int64, commitSHA string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sha, ok := c.reviewed[prID]
	return ok && sha == commitSHA
}

// MarkReviewed records that a PR has been reviewed at a specific commit.
func (c *commitCache) MarkReviewed(prID int64, commitSHA string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reviewed[prID] = commitSHA
}
