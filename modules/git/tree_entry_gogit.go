// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"

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
		Name: te.name,
		Mode: entryModeToGogitFileMode(te.entryMode),
		Hash: plumbing.Hash(te.ID.RawValue()),
	}
}

// GetSize returns the size of the entry
func (te *TreeEntry) GetSize(ctx context.Context, gitRepo *Repository) int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	ptreeGogitTree, err := te.ptree.gogitTreeObject(gitRepo)
	if err != nil {
		return 0
	}
	file, err := ptreeGogitTree.TreeEntryFile(te.toGogitTreeEntry())
	if err != nil {
		return 0
	}

	te.sized = true
	te.size = file.Size
	return te.size
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob(gitRepo *Repository) *Blob {
	return &Blob{
		ID:   te.ID,
		repo: gitRepo,
		name: te.Name(),
	}
}
