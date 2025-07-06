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
	ID         ObjectID
	ResolvedID ObjectID
	repo       *Repository

	gogitTree *object.Tree

	// parent tree
	ptree *Tree
}

func (t *Tree) loadTreeObject() error {
	gogitTree, err := t.repo.gogitRepo.TreeObject(plumbing.Hash(t.ID.RawValue()))
	if err != nil {
		return err
	}

	t.gogitTree = gogitTree
	return nil
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	entries := make([]*TreeEntry, len(t.gogitTree.Entries))
	for i, entry := range t.gogitTree.Entries {
		entries[i] = &TreeEntry{
			ID:             ParseGogitHash(entry.Hash),
			gogitTreeEntry: &t.gogitTree.Entries[i],
			ptree:          t,
		}
	}

	return entries, nil
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	var entries []*TreeEntry
	seen := map[plumbing.Hash]bool{}
	walker := object.NewTreeWalker(t.gogitTree, true, seen)
	for {
		_, entry, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if seen[entry.Hash] {
			continue
		}

		convertedEntry := &TreeEntry{
			ID:             ParseGogitHash(entry.Hash),
			gogitTreeEntry: &entry,
			ptree:          t,
		}
		entries = append(entries, convertedEntry)
	}

	return entries, nil
}

// ListEntriesRecursiveFast is the alias of ListEntriesRecursiveWithSize for the gogit version
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.ListEntriesRecursiveWithSize()
}
