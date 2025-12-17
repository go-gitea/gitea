// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"io"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Tree represents a flat directory listing.
type Tree struct {
	TreeCommon

	resolvedGogitTreeObject *object.Tree
}

func (t *Tree) gogitTreeObject() (_ *object.Tree, err error) {
	if t.resolvedGogitTreeObject == nil {
		t.resolvedGogitTreeObject, err = t.repo.gogitRepo.TreeObject(plumbing.Hash(t.ID.RawValue()))
		if err != nil {
			return nil, err
		}
	}
	return t.resolvedGogitTreeObject, nil
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	gogitTree, err := t.gogitTreeObject()
	if err != nil {
		return nil, err
	}
	entries := make([]*TreeEntry, len(gogitTree.Entries))
	for i, gogitTreeEntry := range gogitTree.Entries {
		entries[i] = &TreeEntry{
			ID:        ParseGogitHash(gogitTreeEntry.Hash),
			ptree:     t,
			name:      gogitTreeEntry.Name,
			entryMode: gogitFileModeToEntryMode(gogitTreeEntry.Mode),
		}
	}

	return entries, nil
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursiveWithSize() (entries Entries, _ error) {
	gogitTree, err := t.gogitTreeObject()
	if err != nil {
		return nil, err
	}

	walker := object.NewTreeWalker(gogitTree, true, nil)
	for {
		fullName, gogitTreeEntry, err := walker.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		convertedEntry := &TreeEntry{
			ID:        ParseGogitHash(gogitTreeEntry.Hash),
			name:      fullName, // FIXME: the "name" field is abused, here it is a full path
			ptree:     t,        // FIXME: this ptree is not right, fortunately it isn't really used
			entryMode: gogitFileModeToEntryMode(gogitTreeEntry.Mode),
		}
		entries = append(entries, convertedEntry)
	}
	return entries, nil
}

// ListEntriesRecursiveFast is the alias of ListEntriesRecursiveWithSize for the gogit version
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.ListEntriesRecursiveWithSize()
}
