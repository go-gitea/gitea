// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// Tree represents a git tree
type Tree interface {
	Object

	//  _
	// |_ ._  _|_ ._ o  _   _
	// |_ | |  |_ |  | (/_ _>
	//

	// GetTreeEntryByPath get the tree entries according the sub dir
	GetTreeEntryByPath(relpath string) (TreeEntry, error)

	// ListEntries returns all entries of current tree.
	ListEntries() (Entries, error)

	// ListEntriesRecursive returns all entries of current tree recursively including all subtrees
	ListEntriesRecursive() (Entries, error)

	// SubTree get a sub tree by the sub dir path
	SubTree(rpath string) (Tree, error)

	// Parent gets the parent tree of this tree (if available)
	Parent() (Tree, error)

	//  _
	// |_) |  _  |_
	// |_) | (_) |_)
	//

	// GetBlobByPath get the blob object according the path
	GetBlobByPath(relpath string) (Blob, error)

	//
	// |_|  _.  _ |_
	// | | (_| _> | |
	//

	// ResolvedID returns the ID was resolved to find this tree
	ResolvedID() Hash
}
