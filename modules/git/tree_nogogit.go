// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// Tree represents a flat directory listing.
type Tree struct {
	TreeCommon

	entries       Entries
	entriesParsed bool
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}

	if t.repo != nil {
		batch, cancel, err := t.repo.CatFileBatch(t.repo.Ctx)
		if err != nil {
			return nil, err
		}
		defer cancel()

		wr := batch.Writer()
		rd := batch.Reader()
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
			t.entries, err = catBatchParseTreeEntries(t.ID.Type(), t, rd, sz)
			if err != nil {
				return nil, err
			}
			t.entriesParsed = true
			return t.entries, nil
		}

		// Not a tree just use ls-tree instead
		if err := DiscardFull(rd, sz+1); err != nil {
			return nil, err
		}
	}

	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-l").AddDynamicArguments(t.ID.String()).WithDir(t.repo.Path).RunStdBytes(t.repo.Ctx)
	if runErr != nil {
		if strings.Contains(runErr.Error(), "fatal: Not a valid object name") || strings.Contains(runErr.Error(), "fatal: not a tree object") {
			return nil, ErrNotExist{
				ID: t.ID.String(),
			}
		}
		return nil, runErr
	}

	var err error
	t.entries, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesParsed = true
	}

	return t.entries, err
}

// listEntriesRecursive returns all entries of current tree recursively including all subtrees
// extraArgs could be "-l" to get the size, which is slower
func (t *Tree) listEntriesRecursive(extraArgs gitcmd.TrustedCmdArgs) (Entries, error) {
	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-t", "-r").
		AddArguments(extraArgs...).
		AddDynamicArguments(t.ID.String()).
		WithDir(t.repo.Path).
		RunStdBytes(t.repo.Ctx)
	if runErr != nil {
		return nil, runErr
	}

	// FIXME: the "name" field is abused, here it is a full path
	// FIXME: this ptree is not right, fortunately it isn't really used
	return parseTreeEntries(stdout, t)
}

// ListEntriesRecursiveFast returns all entries of current tree recursively including all subtrees, no size
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.listEntriesRecursive(nil)
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees, with size
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	return t.listEntriesRecursive(gitcmd.TrustedCmdArgs{"--long"})
}
