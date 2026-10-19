// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"io"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

func newTree(id ObjectID) *Tree {
	return &Tree{ID: id}
}

// SubTree get a subtree by the sub dir path
func (t *Tree) SubTree(ctx context.Context, gitRepo *Repository, rpath string) (*Tree, error) {
	if len(rpath) == 0 {
		return t, nil
	}

	paths := strings.Split(rpath, "/")
	var (
		err error
		g   = t
		p   = t
		te  *TreeEntry
	)
	for _, name := range paths {
		te, err = p.GetTreeEntryByPath(ctx, gitRepo, name)
		if err != nil {
			return nil, err
		}

		g, err = gitRepo.getTree(ctx, te.ID)
		if err != nil {
			return nil, err
		}
		p = g
	}
	return g, nil
}

// LsTree checks if the given filenames are in the tree
func (repo *Repository) LsTree(ctx context.Context, ref string, filenames ...string) ([]string, error) {
	cmd := gitcmd.NewCommand("ls-tree", "-z", "--name-only").
		AddDashesAndList(append([]string{ref}, filenames...)...)

	res, _, err := cmd.WithRepo(repo).RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for line := range bytes.SplitSeq(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

// GetTreePathLatestCommit returns the latest commit of a tree path
func (repo *Repository) GetTreePathLatestCommit(ctx context.Context, refName, treePath string) (*Commit, error) {
	stdout, _, err := gitcmd.NewCommand("rev-list", "-1").
		AddDynamicArguments(refName).AddDashesAndList(treePath).
		WithRepo(repo).
		RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(ctx, strings.TrimSpace(stdout))
}

// Tree represents a flat directory listing.
type Tree struct {
	ID ObjectID

	entries       Entries
	entriesParsed bool
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries(_ctx context.Context, gitRepo *Repository) (Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}

	batch, cancel, err := gitRepo.CatFileBatch()
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
	if info.Type != "tree" { // resolve annotated tags to their tree, a blob yields ErrNotExist
		if err := DiscardFull(rd, info.Size+1); err != nil {
			return nil, err
		}
		info, rd, err = batch.QueryContent(t.ID.String() + "^{tree}")
		if err != nil {
			return nil, err
		}
	}

	t.entries, err = catBatchParseTreeEntries(t.ID.Type(), t, rd, info.Size)
	if err != nil {
		return nil, err
	}
	t.entriesParsed = true
	return t.entries, nil
}

// listEntriesRecursive returns all entries of current tree recursively including all subtrees
// extraArgs could be "-l" to get the size, which is slower
func (t *Tree) listEntriesRecursive(ctx context.Context, gitRepo *Repository, extraArgs gitcmd.TrustedCmdArgs) (Entries, error) {
	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-t", "-r").
		AddArguments(extraArgs...).
		AddDynamicArguments(t.ID.String()).
		WithRepo(gitRepo).
		RunStdBytes(ctx)
	if runErr != nil {
		return nil, runErr
	}

	// FIXME: the "name" field is abused, here it is a full path
	// FIXME: this ptree is not right, fortunately it isn't really used
	return parseTreeEntries(stdout, t)
}

// ListEntriesRecursiveFast returns all entries of current tree recursively including all subtrees, no size
func (t *Tree) ListEntriesRecursiveFast(ctx context.Context, gitRepo *Repository) (Entries, error) {
	return t.listEntriesRecursive(ctx, gitRepo, nil)
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees, with size
func (t *Tree) ListEntriesRecursiveWithSize(ctx context.Context, gitRepo *Repository) (Entries, error) {
	return t.listEntriesRecursive(ctx, gitRepo, gitcmd.TrustedCmdArgs{"--long"})
}
