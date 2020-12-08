// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.TreeEntry) = &TreeEntry{}

// TreeEntry represents an Entry in a Tree
type TreeEntry struct {
	Object

	entryMode service.EntryMode

	ptree service.Tree

	name     string
	fullName string
}

// Name returns the name of the entry
func (te *TreeEntry) Name() string {
	if te.fullName != "" {
		return te.fullName
	}
	return te.name
}

// Mode returns the mode of the entry
// Mode returns the mode of the entry
func (te *TreeEntry) Mode() service.EntryMode {
	return te.entryMode
}

// Tree returns the Tree this Entry is part of
func (te *TreeEntry) Tree() service.Tree {
	return te.ptree
}

// FollowLink returns the entry pointed to by a symlink
func (te *TreeEntry) FollowLink() (service.TreeEntry, error) {
	return common.TreeEntryFollowLink(te)
}

// FollowLinks returns the entry ultimately pointed to by a symlink
func (te *TreeEntry) FollowLinks() (service.TreeEntry, error) {
	return common.TreeEntryFollowLinks(te)
}

// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func (te *TreeEntry) GetSubJumpablePathName() string {
	return common.TreeEntryGetSubJumpablePathName(te)
}
