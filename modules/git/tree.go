// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"strings"
)

// NewTree create a new tree according the repository and tree id
func NewTree(repo *Repository, id ObjectID) *Tree {
	return &Tree{
		ID:   id,
		repo: repo,
	}
}

// SubTree get a subtree by the sub dir path
func (t *Tree) SubTree(rpath string) (*Tree, error) {
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
		te, err = p.GetTreeEntryByPath(name)
		if err != nil {
			return nil, err
		}

		g, err = t.repo.getTree(te.ID)
		if err != nil {
			return nil, err
		}
		g.ptree = p
		p = g
	}
	return g, nil
}

// LsTree checks if the given filenames are in the tree
func (repo *Repository) LsTree(ref string, filenames ...string) ([]string, error) {
	cmd := NewCommand("ls-tree", "-z", "--name-only").
		AddDashesAndList(append([]string{ref}, filenames...)...)

	res, _, err := cmd.RunStdBytes(repo.Ctx, &RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for _, line := range bytes.Split(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

// GetTreePathLatestCommit returns the latest commit of a tree path
func (repo *Repository) GetTreePathLatestCommit(refName, treePath string) (*Commit, error) {
	stdout, _, err := NewCommand("rev-list", "-1").
		AddDynamicArguments(refName).AddDashesAndList(treePath).
		RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(strings.TrimSpace(stdout))
}
