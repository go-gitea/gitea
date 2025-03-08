// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing"
)

func (repo *Repository) getTree(id ObjectID) (*Tree, error) {
	gogitTree, err := repo.gogitRepo.TreeObject(plumbing.Hash(id.RawValue()))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, ErrNotExist{
				ID: id.String(),
			}
		}
		return nil, err
	}

	tree := NewTree(repo, id)
	tree.gogitTree = gogitTree
	return tree, nil
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}

	if len(idStr) != objectFormat.FullLength() {
		res, _, err := NewCommand("rev-parse", "--verify").AddDynamicArguments(idStr).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
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
	resolvedID := id
	commitObject, err := repo.gogitRepo.CommitObject(plumbing.Hash(id.RawValue()))
	if err == nil {
		id = ParseGogitHash(commitObject.TreeHash)
	}
	treeObject, err := repo.getTree(id)
	if err != nil {
		return nil, err
	}
	treeObject.ResolvedID = resolvedID
	return treeObject, nil
}
