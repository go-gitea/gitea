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

		info, rd, err := batch.QueryContent(t.ID.String())
		if err != nil {
			return nil, err
		}

		if info.Type == "commit" {
			treeID, err := ReadTreeID(rd, info.Size)
			if err != nil && err != io.EOF {
				return nil, err
			}
			info, rd, err = batch.QueryContent(treeID)
			if err != nil {
				return nil, err
			}
		}
		if info.Type == "tree" {
			t.entries, err = catBatchParseTreeEntries(t.ID.Type(), rd, info.Size)
			if err != nil {
				return nil, err
			}
			t.entriesParsed = true
			return t.entries, nil
		}

		// Not a tree just use ls-tree instead
		if err := DiscardFull(rd, info.Size+1); err != nil {
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
	t.entries, err = ParseTreeEntries(stdout)
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
	return ParseTreeEntries(stdout)
}

// ListEntriesRecursiveFast returns all entries of current tree recursively including all subtrees, no size
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.listEntriesRecursive(nil)
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees, with size
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	return t.listEntriesRecursive(gitcmd.TrustedCmdArgs{"--long"})
}

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (_ *TreeEntry, err error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID:        t.ID,
			Name:      "",
			EntryMode: EntryModeTree,
		}, nil
	}

	output, _, err := gitcmd.NewCommand("ls-tree", "-l").
		AddDynamicArguments(t.ID.String()).
		WithDir(t.repo.Path).
		AddDashesAndList(relpath).RunStdBytes(t.repo.Ctx)
	if err != nil {
		return nil, err
	}
	if string(output) == "" {
		return nil, ErrNotExist{"", relpath}
	}

	return parseTreeEntry(output)
}
