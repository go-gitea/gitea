// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	gogitTree, err := repo.gogitRepo.TreeObject(plumbing.Hash(id))
	if err != nil {
		return nil, err
	}

	tree := NewTree(repo, id)
	tree.gogitTree = gogitTree
	return tree, nil
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != 40 {
		res, err := NewCommand("rev-parse", idStr).RunInDir(repo.Path)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	commitObject, err := repo.gogitRepo.CommitObject(plumbing.Hash(id))
	if err != nil {
		return nil, err
	}
	treeObject, err := repo.getTree(SHA1(commitObject.TreeHash))
	if err != nil {
		return nil, err
	}
	treeObject.CommitID = id
	return treeObject, nil
}
