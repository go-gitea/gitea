// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// LFSLock represent a lock
// for use with the locks API.
type LFSLock struct {
	// The unique identifier of the lock
	ID string `json:"id"`
	// The file path that is locked
	Path string `json:"path"`
	// The timestamp when the lock was created
	LockedAt time.Time `json:"locked_at"`
	// The owner of the lock
	Owner *LFSLockOwner `json:"owner"`
}

// LFSLockOwner represent a lock owner
// for use with the locks API.
type LFSLockOwner struct {
	// The name of the lock owner
	Name string `json:"name"`
}

// LFSLockRequest contains the path of the lock to create
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#create-lock
type LFSLockRequest struct {
	// The file path to lock
	Path string `json:"path"`
}

// LFSLockResponse represent a lock created
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#create-lock
type LFSLockResponse struct {
	// The created lock
	Lock *LFSLock `json:"lock"`
}

// LFSLockList represent a list of lock requested
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#list-locks
type LFSLockList struct {
	// The list of locks
	Locks []*LFSLock `json:"locks"`
	// The cursor for pagination to the next set of results
	Next string `json:"next_cursor,omitempty"`
}

// LFSLockListVerify represent a list of lock verification requested
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#list-locks-for-verification
type LFSLockListVerify struct {
	// Locks owned by the requesting user
	Ours []*LFSLock `json:"ours"`
	// Locks owned by other users
	Theirs []*LFSLock `json:"theirs"`
	// The cursor for pagination to the next set of results
	Next string `json:"next_cursor,omitempty"`
}

// LFSLockError contains information on the error that occurs
type LFSLockError struct {
	// The error message
	Message string `json:"message"`
	// The lock related to the error, if any
	Lock *LFSLock `json:"lock,omitempty"`
	// URL to documentation about the error
	Documentation string `json:"documentation_url,omitempty"`
	// The request ID for debugging purposes
	RequestID string `json:"request_id,omitempty"`
}

// LFSLockDeleteRequest contains params of a delete request
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#delete-lock
type LFSLockDeleteRequest struct {
	// Whether to force delete the lock even if not owned by the requester
	Force bool `json:"force"`
}
