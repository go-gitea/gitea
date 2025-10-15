// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// NewTree create a new tree according the repository and tree id
func NewTree(repo *Repository, id ObjectID) *Tree {
	return &Tree{
		ID:   id,
		repo: repo,
	}
}

// SubTree get a subtree by the sub dir path
func (t *Tree) SubTree(ctx context.Context, rpath string) (*Tree, error) {
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
		te, err = p.GetTreeEntryByPath(ctx, name)
		if err != nil {
			return nil, err
		}

		g, err = t.repo.getTree(ctx, te.ID)
		if err != nil {
			return nil, err
		}
		g.ptree = p
		p = g
	}
	return g, nil
}

// LsTree checks if the given filenames are in the tree
func (repo *Repository) LsTree(ctx context.Context, ref string, filenames ...string) ([]string, error) {
	cmd := gitcmd.NewCommand("ls-tree", "-z", "--name-only").
		AddDashesAndList(append([]string{ref}, filenames...)...)

	res, _, err := cmd.WithDir(repo.Path).RunStdBytes(ctx)
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
		WithDir(repo.Path).
		RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(ctx, strings.TrimSpace(stdout))
}
