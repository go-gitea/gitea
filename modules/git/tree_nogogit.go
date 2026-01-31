// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
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

		currentID := t.ID.String()
		for {
			var (
				objectType string
				treeID     string
			)
			err = batch.QueryContent(currentID, func(info *CatFileObject, reader io.Reader) error {
				objectType = info.Type
				switch info.Type {
				case "commit":
					bufReader := bufio.NewReader(reader)
					var err error
					treeID, err = ReadTreeID(bufReader)
					if err != nil && err != io.EOF {
						return err
					}
				case "tree":
					objectFormat := t.ID.Type()
					t.entries, err = catBatchParseTreeEntries(objectFormat, t, reader, info.Size)
					if err != nil {
						return err
					}
					t.entriesParsed = true
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			if objectType == "commit" && treeID != "" {
				currentID = treeID
				continue
			}
			if objectType == "tree" && t.entriesParsed {
				return t.entries, nil
			}
			break
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
