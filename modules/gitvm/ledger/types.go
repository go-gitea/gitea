// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

// Receipt represents a tamper-evident event record in the GitVM ledger
type Receipt struct {
	Version     int         `json:"v"`
	Type        string      `json:"type"`        // e.g. "git.push", "pr.merged", "release.published", "perm.changed"
	TsUnixMs    int64       `json:"ts_unix_ms"`  // stable + sortable timestamp
	Repo        RepoRef     `json:"repo"`
	Actor       ActorRef    `json:"actor"`
	Payload     interface{} `json:"payload"` // typed per event
	PrevRoot    string      `json:"prev_root"`
	ReceiptHash string      `json:"receipt_hash"` // set after hashing canonical bytes
	Root        string      `json:"root"`         // rolling root after this receipt
}

// RepoRef identifies a repository
type RepoRef struct {
	ID   int64  `json:"id"`
	Full string `json:"full"` // "owner/name"
}

// ActorRef identifies a user/actor
type ActorRef struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// PushPayload captures git push event data
type PushPayload struct {
	Ref    string `json:"ref"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// PRMergedPayload captures pull request merge event data
type PRMergedPayload struct {
	PRID        int64  `json:"pr_id"`
	Base        string `json:"base"`
	Head        string `json:"head"`
	MergeCommit string `json:"merge_commit"`
}

// ReleasePayload captures release publish event data
type ReleasePayload struct {
	Tag           string `json:"tag"`
	ReleaseID     int64  `json:"release_id"`
	ArtifactsHash string `json:"artifacts_hash,omitempty"` // future: hash of release artifacts
}

// PermissionPayload captures permission change event data
type PermissionPayload struct {
	SubjectType string `json:"subject_type"` // "user" | "team"
	SubjectID   int64  `json:"subject_id"`
	Permission  string `json:"permission"` // "read" | "write" | "admin"
}

// CIRunStartPayload captures CI run start event
type CIRunStartPayload struct {
	RunID      int64  `json:"run_id"`
	WorkflowID string `json:"workflow_id"` // workflow filename
	CommitSHA  string `json:"commit_sha"`
	Ref        string `json:"ref"`   // branch/tag/pr ref
	Event      string `json:"event"` // push/pull_request/manual/...
}

// CIRunEndPayload captures CI run completion event
type CIRunEndPayload struct {
	RunID      int64  `json:"run_id"`
	Status     string `json:"status"`      // success/failure/cancelled/skipped
	DurationMs int64  `json:"duration_ms"` // run duration in milliseconds
	CommitSHA  string `json:"commit_sha"`  // commit that was tested
	WorkflowID string `json:"workflow_id"` // workflow filename
	Ref        string `json:"ref"`         // branch/tag/pr ref
	Event      string `json:"event"`       // push/pull_request/manual/...
}
