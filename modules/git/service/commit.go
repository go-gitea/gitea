// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"container/list"
)

// Commit represents a git commit
type Commit interface {
	Object

	// Tree returns the tree that this commit references to
	Tree() Tree

	// TreeID returns the hash for this commit tree
	TreeID() Hash

	// Author returns the author signature
	Author() *Signature

	// Committer returns the committer signature
	Committer() *Signature

	// Message returns the commit message. Same as retrieving CommitMessage directly.
	Message() string

	// Summary returns first line of commit message.
	Summary() string

	// Parents returns the parent hashes
	Parents() []Hash

	// ParentID returns oid of n-th parent (0-based index).
	// It returns nil if no such parent exists.
	ParentID(n int) (Hash, error)

	// Parent returns n-th parent (0-based index) of the commit.
	Parent(n int) (Commit, error)

	// ParentCount returns number of parents of the commit.
	// 0 if this is a root commit,  otherwise 1,2, etc.
	ParentCount() int

	// Signature returns the GPGSignature for this commit
	Signature() *GPGSignature

	//  _
	// /   _  ._ _  ._ _  o _|_
	// \_ (_) | | | | | | |  |_
	//

	// CommitsCount returns number of total commits of until current revision.
	CommitsCount() (int64, error)

	// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
	CommitsByRange(page, pageSize int) (*list.List, error)

	// CommitsBefore returns all the commits before current revision
	CommitsBefore() (*list.List, error)

	// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
	HasPreviousCommit(commitHash Hash) (bool, error)

	// CommitsBeforeLimit returns num commits before current revision
	CommitsBeforeLimit(num int) (*list.List, error)

	// CommitsBeforeUntil returns the commits between commitID to current revision
	CommitsBeforeUntil(commitID string) (*list.List, error)

	// SearchCommits returns the commits match the keyword before current revision
	SearchCommits(opts SearchCommitsOptions) (*list.List, error)

	//  _
	// |_ o |  _
	// |  | | (/_
	//

	// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
	GetFilesChangedSinceCommit(pastCommit string) ([]string, error)

	// FileChangedSinceCommit Returns true if the file given has changed since the the past commit
	// YOU MUST ENSURE THAT pastCommit is a valid commit ID.
	FileChangedSinceCommit(filename, pastCommit string) (bool, error)

	// HasFile returns true if the file given exists on this commit
	// This does only mean it's there - it does not mean the file was changed during the commit.
	HasFile(filename string) (bool, error)

	// GetSubModules get all the sub modules of current revision git tree
	GetSubModules() (ObjectCache, error)

	// GetSubModule get the sub module according entryname
	GetSubModule(entryname string) (*SubModule, error)

	//  _
	// |_) ._  _. ._   _ |_
	// |_) |  (_| | | (_ | |
	//

	// Branch returns the branch that this commit is on
	Branch() string

	// GetBranchName gets the closest branch name (as returned by 'git name-rev --name-only')
	GetBranchName() (string, error)

	// LoadBranchName load branch name for commit
	LoadBranchName() (err error)

	// GetTagName gets the current tag name for given commit
	GetTagName() (string, error)

	//  __  _   __
	// /__ |_) /__
	// \_| |   \_|
	//

	// GetRepositoryDefaultPublicGPGKey returns the default public key for this commit
	GetRepositoryDefaultPublicGPGKey(forceUpdate bool) (*GPGSettings, error)
}
