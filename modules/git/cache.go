// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// LastCommitCache cache
type LastCommitCache interface {
	Get(repoPath, ref, entryPath string) (*Commit, error)
	Put(repoPath, ref, entryPath string, commit *Commit) error
}
