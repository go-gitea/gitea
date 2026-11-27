// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// swagger:enum MergeQueueEntryState
type MergeQueueEntryState string

const (
	MergeQueueStateQueued         MergeQueueEntryState = "queued"
	MergeQueueStateAwaitingChecks MergeQueueEntryState = "awaiting_checks"
	MergeQueueStateMergeable      MergeQueueEntryState = "mergeable"
	MergeQueueStateUnmergeable    MergeQueueEntryState = "unmergeable"
)

// MergeQueueEntry is a pull request waiting in a merge queue
type MergeQueueEntry struct {
	// Position in the queue, starting at 1
	Position int `json:"position"`
	// queued, awaiting_checks, mergeable or unmergeable
	State MergeQueueEntryState `json:"state"`
	// Index of the pull request
	PullRequestIndex int64 `json:"pull_request_index"`
	// Branch the pull request merges into
	BaseBranch string `json:"base_branch"`
	// Head commit of the pull request when it was added
	HeadSHA string `json:"head_sha"`
	// Queue commit testing the pull request, empty until it is built
	MergeGroupHeadSHA string `json:"merge_group_head_sha"`
	// Ref of the queue commit, empty until it is built
	MergeGroupHeadRef string `json:"merge_group_head_ref"`
	// Whether the pull request jumped to the front of the queue
	Jump bool `json:"jump"`
	// User who added the pull request
	Enqueuer *User `json:"enqueuer"`
	// swagger:strfmt date-time
	EnqueuedAt time.Time `json:"enqueued_at"`
}

// AddToMergeQueueOption options for adding a pull request to the merge queue
type AddToMergeQueueOption struct {
	// Put the pull request at the front of the queue, even if already queued, requires repository admin
	Jump bool `json:"jump"`
	// Only add the pull request if its head is still this commit
	HeadCommitID string `json:"head_commit_id"`
}
