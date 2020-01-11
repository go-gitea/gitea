// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"

	"xorm.io/xorm"
)

// ReleaseV39 describes the added field for Release
type ReleaseV39 struct {
	IsTag bool `xorm:"NOT NULL DEFAULT false"`
}

// TableName will be invoked by XORM to customrize the table name
func (*ReleaseV39) TableName() string {
	return "release"
}

func releaseAddColumnIsTagAndSyncTags(x *xorm.Engine) error {
	if err := x.Sync2(new(ReleaseV39)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	// For the sake of SQLite3, we can't use x.Iterate here.
	offset := 0
	pageSize := models.RepositoryListDefaultPageSize
	for {
		repos := make([]*models.Repository, 0, pageSize)
		if err := x.Table("repository").Cols("id", "name", "owner_id").Asc("id").Limit(pageSize, offset).Find(&repos); err != nil {
			return fmt.Errorf("select repos [offset: %d]: %v", offset, err)
		}
		for _, repo := range repos {
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				log.Warn("OpenRepository: %v", err)
				continue
			}

			if err = repository.SyncReleasesWithTags(repo, gitRepo); err != nil {
				log.Warn("SyncReleasesWithTags: %v", err)
			}
			gitRepo.Close()
		}
		if len(repos) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}
