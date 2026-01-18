// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"io"
	"path"

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
			name:      gogitTreeEntry.Name,
			EntryMode: gogitFileModeToEntryMode(gogitTreeEntry.Mode),
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
			EntryMode: gogitFileModeToEntryMode(gogitTreeEntry.Mode),
		}
		entries = append(entries, convertedEntry)
	}
	return entries, nil
}

// ListEntriesRecursiveFast is the alias of ListEntriesRecursiveWithSize for the gogit version
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.ListEntriesRecursiveWithSize()
}

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID:        t.ID,
			name:      "",
			EntryMode: EntryModeTree,
		}, nil
	}

	gogitTree, err := t.gogitTreeObject()
	if err != nil {
		return nil, err
	}

	relpath = path.Clean(relpath)
	e, err := gogitTree.FindEntry(relpath)
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			return nil, ErrNotExist{
				RelPath: relpath,
			}
		}
		return nil, err
	}

	return &TreeEntry{
		ID:        ParseGogitHash(e.Hash),
		name:      path.Base(relpath),
		EntryMode: EntryMode(e.Mode),
	}, nil
}
