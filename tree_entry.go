// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	ENTRY_MODE_BLOB    EntryMode = 0100644
	ENTRY_MODE_EXEC    EntryMode = 0100755
	ENTRY_MODE_SYMLINK EntryMode = 0120000
	ENTRY_MODE_COMMIT  EntryMode = 0160000
	ENTRY_MODE_TREE    EntryMode = 0040000
)

type TreeEntry struct {
	ID   sha1
	Type ObjectType

	mode EntryMode
	name string

	ptree *Tree

	commited bool

	size  int64
	sized bool
}

func (te *TreeEntry) IsDir() bool {
	return te.mode == ENTRY_MODE_TREE
}

func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		repo:      te.ptree.repo,
		TreeEntry: te,
	}
}

type Entries []*TreeEntry
