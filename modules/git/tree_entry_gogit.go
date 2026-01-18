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
		Name: te.name,
		Mode: entryModeToGogitFileMode(te.entryMode),
		Hash: plumbing.Hash(te.ID.RawValue()),
	}
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		ID:   te.ID,
		repo: te.ptree.repo,
		name: te.Name(),
	}
}
