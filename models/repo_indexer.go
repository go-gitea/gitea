// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"xorm.io/builder"
)

// RepoIndexerStatus status of a repo's entry in the repo indexer
// For now, implicitly refers to default branch
type RepoIndexerStatus struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX"`
	CommitSha string `xorm:"VARCHAR(40)"`
}

// GetUnindexedRepos returns repos which do not have an indexer status
func GetUnindexedRepos(maxRepoID int64, page, pageSize int) ([]int64, error) {
	ids := make([]int64, 0, 50)
	cond := builder.Cond(builder.IsNull{
		"repo_indexer_status.id",
	})
	sess := x.Table("repository").Join("LEFT OUTER", "repo_indexer_status", "repository.id = repo_indexer_status.repo_id")
	if maxRepoID > 0 {
		cond = builder.And(cond, builder.Lte{
			"repository.id": maxRepoID,
		})
	}
	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}

	sess.Where(cond).Cols("repository.id").Desc("repository.id")
	err := sess.Find(&ids)
	return ids, err
}

// GetIndexerStatus loads repo codes indxer status
func (repo *Repository) GetIndexerStatus() error {
	if repo.IndexerStatus != nil {
		return nil
	}
	status := &RepoIndexerStatus{RepoID: repo.ID}
	has, err := x.Get(status)
	if err != nil {
		return err
	} else if !has {
		status.CommitSha = ""
	}
	repo.IndexerStatus = status
	return nil
}

// UpdateIndexerStatus updates indexer status
func (repo *Repository) UpdateIndexerStatus(sha string) error {
	if err := repo.GetIndexerStatus(); err != nil {
		return fmt.Errorf("UpdateIndexerStatus: Unable to getIndexerStatus for repo: %s/%s Error: %v", repo.MustOwnerName(), repo.Name, err)
	}
	if len(repo.IndexerStatus.CommitSha) == 0 {
		repo.IndexerStatus.CommitSha = sha
		_, err := x.Insert(repo.IndexerStatus)
		if err != nil {
			return fmt.Errorf("UpdateIndexerStatus: Unable to insert repoIndexerStatus for repo: %s/%s Sha: %s Error: %v", repo.MustOwnerName(), repo.Name, sha, err)
		}
		return nil
	}
	repo.IndexerStatus.CommitSha = sha
	_, err := x.ID(repo.IndexerStatus.ID).Cols("commit_sha").
		Update(repo.IndexerStatus)
	if err != nil {
		return fmt.Errorf("UpdateIndexerStatus: Unable to update repoIndexerStatus for repo: %s/%s Sha: %s Error: %v", repo.MustOwnerName(), repo.Name, sha, err)
	}
	return nil
}
