// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"

	repo_model "code.gitea.io/gitea/models/repo"
)

// LoadArchiverRepo loads repository
func LoadArchiverRepo(archiver *repo_model.RepoArchiver) (*Repository, error) {
	var repo Repository
	has, err := db.GetEngine(db.DefaultContext).ID(archiver.RepoID).Get(&repo)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrRepoNotExist{
			ID: archiver.RepoID,
		}
	}
	return &repo, nil
}
