// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"

	"code.gitea.io/gitea/modules/git/catfile"
)

func (repo *Repository) getTree(id ObjectID) (*Tree, error) {
	objectPool, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	object, rd, err := objectPool.Object(id.String())
	if err != nil {
		if catfile.IsErrObjectNotFound(err) {
			return nil, ErrNotExist{
				ID: id.String(),
			}
		}
		return nil, err
	}

	switch object.Type {
	case "tag":
		resolvedID := id
		data, err := io.ReadAll(io.LimitReader(rd, object.Size))
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(id.Type(), data)
		if err != nil {
			return nil, err
		}

		commit, err := repo.getCommitFromBatchReader(objectPool, tag.Object)
		if err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = resolvedID
		return &commit.Tree, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, object.Size))
		if err != nil {
			return nil, err
		}
		if _, err := rd.Discard(1); err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = commit.ID
		return &commit.Tree, nil
	case "tree":
		tree := NewTree(repo, id)
		tree.ResolvedID = id
		objectFormat, err := repo.GetObjectFormat()
		if err != nil {
			return nil, err
		}
		tree.entries, err = catBatchParseTreeEntries(objectFormat, tree, rd, object.Size)
		if err != nil {
			return nil, err
		}
		tree.entriesParsed = true
		return tree, nil
	default:
		if err := DiscardFull(rd, object.Size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	if len(idStr) != objectFormat.FullLength() {
		res, err := repo.GetRefCommitID(idStr)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	return repo.getTree(id)
}
