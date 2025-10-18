// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// Tree represents a flat directory listing.
type Tree struct {
	ID         ObjectID
	ResolvedID ObjectID
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
		wr, rd, cancel, err := t.repo.CatFileBatch(t.repo.Ctx)
		if err != nil {
			return nil, err
		}
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
	if t.entriesRecursiveParsed {
		return t.entriesRecursive, nil
	}

	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-t", "-r").
		AddArguments(extraArgs...).
		AddDynamicArguments(t.ID.String()).
		WithDir(t.repo.Path).
		RunStdBytes(t.repo.Ctx)
	if runErr != nil {
		return nil, runErr
	}

	var err error
	t.entriesRecursive, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesRecursiveParsed = true
	}

	return t.entriesRecursive, err
}

// ListEntriesRecursiveFast returns all entries of current tree recursively including all subtrees, no size
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.listEntriesRecursive(nil)
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees, with size
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	return t.listEntriesRecursive(gitcmd.TrustedCmdArgs{"--long"})
}

// IterateEntriesRecursive returns iterate entries of current tree recursively including all subtrees
// extraArgs could be "-l" to get the size, which is slower
func (t *Tree) IterateEntriesRecursive(ctx context.Context, f func(ctx context.Context, entry *TreeEntry) error, extraArgs TrustedCmdArgs) error {
	reader, writer := io.Pipe()
	done := make(chan error)

	go func(t *Tree, done chan error, writer *io.PipeWriter) {
		runErr := NewCommand("ls-tree", "-t", "-r").
			AddArguments(extraArgs...).
			AddDynamicArguments(t.ID.String()).
			Run(ctx, &RunOpts{
				Dir:    t.repo.Path,
				Stdout: writer,
			})

		_ = writer.Close()

		done <- runErr
	}(t, done, writer)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}

		data := scanner.Bytes()
		if err := iterateTreeEntries(data, t, func(entry *TreeEntry) error {
			if err := f(ctx, entry); err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case runErr := <-done:
				return runErr
			default:
				return nil
			}
		}); err != nil {
			return err
		}
	}
	return nil
}
