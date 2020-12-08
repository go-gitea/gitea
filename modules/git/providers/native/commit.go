// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

var _ (service.Commit) = &Commit{}

// Commit represents a git commit
type Commit struct {
	service.Object

	treeID service.Hash
	ptree  service.Tree

	author    *service.Signature
	committer *service.Signature

	signature *service.GPGSignature

	parents        []service.Hash
	submoduleCache *git.ObjectCache

	message string

	branch string
}

// NewCommit creates a commit from the provided data
func NewCommit(object service.Object,
	treeID service.Hash,
	tree service.Tree,
	author, committer *service.Signature,
	signature *service.GPGSignature,
	parents []service.Hash,
	message string) service.Commit {
	return &Commit{
		Object:    object,
		treeID:    treeID,
		ptree:     tree,
		author:    author,
		committer: committer,
		signature: signature,
		parents:   parents,
		message:   message,
	}
}

// SetObject sets the object of this commit
func (commit *Commit) SetObject(object service.Object) {
	commit.Object = object
}

// SetTree sets the tree of the commit
func (commit *Commit) SetTree(tree service.Tree) {
	commit.ptree = tree
	commit.treeID = tree.ID()
}

// SetTreeID sets the tree of the commit
func (commit *Commit) SetTreeID(treeID service.Hash) {
	commit.treeID = treeID
	if commit.ptree != nil && commit.ptree.ID().String() != commit.treeID.String() {
		commit.ptree = nil
	}
}

// SetAuthor sets the author of the commit
func (commit *Commit) SetAuthor(author *service.Signature) {
	commit.author = author
}

// SetCommitter sets the committer of the commit
func (commit *Commit) SetCommitter(committer *service.Signature) {
	commit.committer = committer
}

// SetParents sets the parents of the commit
func (commit *Commit) SetParents(parents []service.Hash) {
	commit.parents = parents
}

// SetSignature sets the signature of the commit
func (commit *Commit) SetSignature(signature *service.GPGSignature) {
	commit.signature = signature
}

// SetMessage sets the message of the commit
func (commit *Commit) SetMessage(message string) {
	commit.message = message
}

// SetBranch sets the branch of the commit
func (commit *Commit) SetBranch(branch string) {
	commit.branch = branch
}

// Tree returns the tree that this commit references to
func (commit *Commit) Tree() service.Tree {
	// FIXME: this should check if the Tree is nil and if so load it
	if commit.ptree == nil {
		var err error
		commit.ptree, err = commit.Repository().GetTree(commit.treeID.String())
		if err != nil {
			log.Error("Unable to load tree %s for commit %s", commit.treeID.String(), commit.ID().String())
		}
	}
	return commit.ptree
}

// TreeID returns the hash for this commit tree
func (commit *Commit) TreeID() service.Hash {
	return commit.treeID
}

// Author returns the author signature
func (commit *Commit) Author() *service.Signature {
	return commit.author
}

// Committer returns the committer signature
func (commit *Commit) Committer() *service.Signature {
	return commit.committer
}

// Message returns the commit message. Same as retrieving CommitMessage directly.
func (commit *Commit) Message() string {
	return commit.message
}

// Summary returns first line of commit message.
func (commit *Commit) Summary() string {
	return strings.SplitN(strings.TrimSpace(commit.message), "\n", 2)[0]
}

// Parents returns the parent hashes
func (commit *Commit) Parents() []service.Hash {
	return commit.parents
}

// ParentID returns oid of n-th parent (0-based index).
// It returns nil if no such parent exists.
func (commit *Commit) ParentID(n int) (service.Hash, error) {
	if n >= len(commit.parents) {
		return StringHash(""), git.ErrNotExist{ID: "", RelPath: ""}
	}
	return commit.parents[n], nil
}

// Parent returns n-th parent (0-based index) of the commit.
func (commit *Commit) Parent(n int) (service.Commit, error) {
	id, err := commit.ParentID(n)
	if err != nil {
		return nil, err
	}
	parent, err := commit.Repository().GetCommit(id.String())
	if err != nil {
		return nil, err
	}
	return parent, nil
}

// ParentCount returns number of parents of the commit.
// 0 if this is a root commit,  otherwise 1,2, etc.
func (commit *Commit) ParentCount() int {
	return len(commit.parents)
}

// Signature returns the GPG signature associated with the commit
func (commit *Commit) Signature() *service.GPGSignature {
	return commit.signature
}
