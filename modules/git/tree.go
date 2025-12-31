// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
)

type TreeCommon struct {
	ID         ObjectID
	ResolvedID ObjectID

	repo  *Repository
	ptree *Tree // parent tree
}

// NewTree create a new tree according the repository and tree id
func NewTree(repo *Repository, id ObjectID) *Tree {
	return &Tree{
		TreeCommon: TreeCommon{
			ID:   id,
			repo: repo,
		},
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
