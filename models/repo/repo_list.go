// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
)

// GetUserMirrorRepositories returns a list of mirror repositories of given user.
func GetUserMirrorRepositories(userID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, db.GetEngine(db.DefaultContext).
		Where("owner_id = ?", userID).
		And("is_mirror = ?", true).
		Find(&repos)
}

// IterateRepository iterate repositories
func IterateRepository(f func(repo *Repository) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	for {
		repos := make([]*Repository, 0, batchSize)
		if err := db.GetEngine(db.DefaultContext).Limit(batchSize, start).Find(&repos); err != nil {
			return err
		}
		if len(repos) == 0 {
			return nil
		}
		start += len(repos)

		for _, repo := range repos {
			if err := f(repo); err != nil {
				return err
			}
		}
	}
}

// FindReposMapByIDs find repos as map
func FindReposMapByIDs(repoIDs []int64, res map[int64]*Repository) error {
	return db.GetEngine(db.DefaultContext).In("id", repoIDs).Find(&res)
}
