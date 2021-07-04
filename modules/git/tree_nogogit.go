// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"io"
	"math"
	"strings"
)

// Tree represents a flat directory listing.
type Tree struct {
	ID         SHA1
	ResolvedID SHA1
	repo       *Repository

	// parent tree
	ptree *Tree

	entries       Entries
	entriesParsed bool

	entriesRecursive       Entries
	entriesRecursiveParsed bool
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}

	if t.repo != nil {
		wr, rd, cancel := t.repo.CatFileBatch()
		defer cancel()

		_, _ = wr.Write([]byte(t.ID.String() + "\n"))
		_, typ, sz, err := ReadBatchLine(rd)
		if err != nil {
			return nil, err
		}
		if typ == "commit" {
			treeID, err := ReadTreeID(rd, sz)
			if err != nil && err != io.EOF {
				return nil, err
			}
			_, _ = wr.Write([]byte(treeID + "\n"))
			_, typ, sz, err = ReadBatchLine(rd)
			if err != nil {
				return nil, err
			}
		}
		if typ == "tree" {
			t.entries, err = catBatchParseTreeEntries(t, rd, sz)
			if err != nil {
				return nil, err
			}
			t.entriesParsed = true
			return t.entries, nil
		}

		// Not a tree just use ls-tree instead
		for sz > math.MaxInt32 {
			discarded, err := rd.Discard(math.MaxInt32)
			sz -= int64(discarded)
			if err != nil {
				return nil, err
			}
		}
		for sz > 0 {
			discarded, err := rd.Discard(int(sz))
			sz -= int64(discarded)
			if err != nil {
				return nil, err
			}
		}
	}

	stdout, err := NewCommand("ls-tree", "-l", t.ID.String()).RunInDirBytes(t.repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") || strings.Contains(err.Error(), "fatal: not a tree object") {
			return nil, ErrNotExist{
				ID: t.ID.String(),
			}
		}
		return nil, err
	}

	t.entries, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesParsed = true
	}

	return t.entries, err
}

// ListEntriesRecursive returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursive() (Entries, error) {
	if t.entriesRecursiveParsed {
		return t.entriesRecursive, nil
	}
	stdout, err := NewCommand("ls-tree", "-t", "-l", "-r", t.ID.String()).RunInDirBytes(t.repo.Path)
	if err != nil {
		return nil, err
	}

	t.entriesRecursive, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesRecursiveParsed = true
	}

	return t.entriesRecursive, err
}
