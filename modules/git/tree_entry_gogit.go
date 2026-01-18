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

// gogitFileModeToEntryMode converts go-git filemode to EntryMode
func gogitFileModeToEntryMode(mode filemode.FileMode) EntryMode {
	return EntryMode(mode)
}

func entryModeToGogitFileMode(mode EntryMode) filemode.FileMode {
	return filemode.FileMode(mode)
}

func (te *TreeEntry) toGogitTreeEntry() *object.TreeEntry {
	return &object.TreeEntry{
		Name: te.Name,
		Mode: entryModeToGogitFileMode(te.EntryMode),
		Hash: plumbing.Hash(te.ID.RawValue()),
	}
}

// GetSize returns the size of the entry
func (te *TreeEntry) GetSize(repo *Repository) int64 {
	if te.IsDir() {
		return 0
	} else if te.Sized {
		return te.Size
	}

	ptreeGogitTree, err := repo.gogitRepo.TreeObject(plumbing.Hash(te.ID.RawValue()))
	if err != nil {
		return 0
	}
	file, err := ptreeGogitTree.TreeEntryFile(te.toGogitTreeEntry())
	if err != nil {
		return 0
	}

	te.Sized = true
	te.Size = file.Size
	return te.Size
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		ID:   te.ID,
		name: te.Name,
	}
}
