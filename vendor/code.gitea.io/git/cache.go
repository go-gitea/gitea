// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// LastCommitCache is the cache interface for get last commit info of an entry on the git tree
type LastCommitCache interface {
	Get(repoPath, ref, entryPath string) (*Commit, error)
	Put(repoPath, ref, entryPath string, commit *Commit) error
}

// LsTreeCache is the cache interface for get a tree entries
type LsTreeCache interface {
	Get(repoPath, treeIsh string) (Entries, error)
	Put(repoPath, treeIsh string, entries Entries) error
}
