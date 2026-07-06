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
	mu        sync.RWMutex
	states    map[int64]ReviewStatus // PRID → status
	counts    map[int64]int          // PRID → issue count
	dismissed map[int64]map[string]bool // PRID → set of "file:line" dismissed findings
}

var reviewStatus = &StatusStore{
	states:    make(map[int64]ReviewStatus),
	counts:    make(map[int64]int),
	dismissed: make(map[int64]map[string]bool),
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

// DismissFinding marks a finding as dismissed for a PR.
func DismissFinding(prID int64, file string, line int) {
	reviewStatus.mu.Lock()
	defer reviewStatus.mu.Unlock()
	key := file + ":" + itoa(line)
	if _, ok := reviewStatus.dismissed[prID]; !ok {
		reviewStatus.dismissed[prID] = make(map[string]bool)
	}
	reviewStatus.dismissed[prID][key] = true
}

// IsFindingDismissed checks if a finding has been dismissed.
func IsFindingDismissed(prID int64, file string, line int) bool {
	reviewStatus.mu.RLock()
	defer reviewStatus.mu.RUnlock()
	key := file + ":" + itoa(line)
	return reviewStatus.dismissed[prID] != nil && reviewStatus.dismissed[prID][key]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(byte('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
