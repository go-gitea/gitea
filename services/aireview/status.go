// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import "sync"

// ReviewStatus represents the state of an AI review.
type ReviewStatus string

const (
	StatusPending   ReviewStatus = "pending"
	StatusRunning   ReviewStatus = "running"
	StatusCompleted ReviewStatus = "completed"
	StatusIssues    ReviewStatus = "issues_found"
	StatusError     ReviewStatus = "error"
)

// StatusStore tracks review statuses per PR.
type StatusStore struct {
	mu     sync.RWMutex
	states map[int64]ReviewStatus  // PRID → status
	counts map[int64]int           // PRID → issue count
}

var reviewStatus = &StatusStore{
	states: make(map[int64]ReviewStatus),
	counts: make(map[int64]int),
}

// SetReviewStatus updates the review status for a PR.
func SetReviewStatus(prID int64, status ReviewStatus, issueCount int) {
	reviewStatus.mu.Lock()
	defer reviewStatus.mu.Unlock()
	reviewStatus.states[prID] = status
	reviewStatus.counts[prID] = issueCount
}

// GetReviewStatus returns the review status and issue count for a PR.
func GetReviewStatus(prID int64) (ReviewStatus, int) {
	reviewStatus.mu.RLock()
	defer reviewStatus.mu.RUnlock()
	return reviewStatus.states[prID], reviewStatus.counts[prID]
}
