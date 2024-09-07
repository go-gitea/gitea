// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/git"
)

// LFSResult represents commits found using a provided pointer file hash
type LFSResult struct {
	Name           string
	SHA            string
	Summary        string
	When           time.Time
	ParentHashes   []git.ObjectID
	BranchName     string
	FullCommitName string
}

type lfsResultSlice []*LFSResult

func (a lfsResultSlice) Len() int           { return len(a) }
func (a lfsResultSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a lfsResultSlice) Less(i, j int) bool { return a[j].When.After(a[i].When) }

func lfsError(msg string, err error) error {
	return fmt.Errorf("LFS error occurred, %s: err: %w", msg, err)
}
