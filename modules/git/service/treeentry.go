// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// TreeEntry represents an Entry in a Tree
type TreeEntry interface {
	Object

	// Name returns the name of the entry
	Name() string

	// Mode returns the mode of the entry
	Mode() EntryMode

	// FollowLink returns the entry pointed to by a symlink
	FollowLink() (TreeEntry, error)

	// FollowLinks returns the entry ultimately pointed to by a symlink
	FollowLinks() (TreeEntry, error)

	// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
	GetSubJumpablePathName() string

	// Tree returns the Tree associated with this TreeEntry if known
	Tree() Tree
}
