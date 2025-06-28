// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// TreeEntry the leaf in the git tree
type TreeEntry struct {
	ID ObjectID

	gogitTreeEntry *object.TreeEntry
	ptree          *Tree

	size  int64
	sized bool
}

// Name returns the name of the entry
func (te *TreeEntry) Name() string {
	return te.gogitTreeEntry.Name
}

// Mode returns the mode of the entry
func (te *TreeEntry) Mode() EntryMode {
	return EntryMode(te.gogitTreeEntry.Mode)
}

// Size returns the size of the entry
func (te *TreeEntry) Size() int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	file, err := te.ptree.gogitTree.TreeEntryFile(te.gogitTreeEntry)
	if err != nil {
		return 0
	}

	te.sized = true
	te.size = file.Size
	return te.size
}

// IsSubModule if the entry is a submodule
func (te *TreeEntry) IsSubModule() bool {
	return te.gogitTreeEntry.Mode == filemode.Submodule
}

// IsDir if the entry is a sub dir
func (te *TreeEntry) IsDir() bool {
	return te.gogitTreeEntry.Mode == filemode.Dir
}

// IsLink if the entry is a symlink
func (te *TreeEntry) IsLink() bool {
	return te.gogitTreeEntry.Mode == filemode.Symlink
}

// IsRegular if the entry is a regular file
func (te *TreeEntry) IsRegular() bool {
	return te.gogitTreeEntry.Mode == filemode.Regular
}

// IsExecutable if the entry is an executable file (not necessarily binary)
func (te *TreeEntry) IsExecutable() bool {
	return te.gogitTreeEntry.Mode == filemode.Executable
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	encodedObj, err := te.ptree.repo.gogitRepo.Storer.EncodedObject(plumbing.AnyObject, te.gogitTreeEntry.Hash)
	if err != nil {
		return nil
	}

	return &Blob{
		ID:              ParseGogitHash(te.gogitTreeEntry.Hash),
		gogitEncodedObj: encodedObj,
		name:            te.Name(),
	}
}
