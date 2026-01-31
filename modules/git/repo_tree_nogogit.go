// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"
)

func (repo *Repository) getTree(id ObjectID) (*Tree, error) {
	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	resolvedID := id
	var (
		objectType string
		tagObject  ObjectID
		commit     *Commit
		tree       *Tree
	)

	err = batch.QueryContent(id.String(), func(info *CatFileObject, reader io.Reader) error {
		objectType = info.Type
		switch info.Type {
		case "tag":
			data, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			tag, err := parseTagData(id.Type(), data)
			if err != nil {
				return err
			}
			tagObject = tag.Object
		case "commit":
			var err error
			commit, err = CommitFromReader(repo, id, reader)
			if err != nil {
				return err
			}
		case "tree":
			tree = NewTree(repo, id)
			tree.ResolvedID = id
			objectFormat := id.Type()
			tree.entries, err = catBatchParseTreeEntries(objectFormat, tree, reader, info.Size)
			if err != nil {
				return err
			}
			tree.entriesParsed = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	switch objectType {
	case "tag":
		commit, err := repo.getCommitWithBatch(batch, tagObject)
		if err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = resolvedID
		return &commit.Tree, nil
	case "commit":
		commit.Tree.ResolvedID = commit.ID
		return &commit.Tree, nil
	case "tree":
		return tree, nil
	default:
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
