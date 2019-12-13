// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
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
	sess := x.Table("repo").Join("LEFT OUTER", "repo_indexer_status", "repo.id = repo_indexer_status.repoID")
	if maxRepoID > 0 {
		cond = builder.And(cond, builder.Lte{
			"repo.id": maxRepoID,
		})
	}
	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}

	sess.Where(cond).Cols("repo.id")
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
		return err
	}
	if len(repo.IndexerStatus.CommitSha) == 0 {
		repo.IndexerStatus.CommitSha = sha
		_, err := x.Insert(repo.IndexerStatus)
		return err
	}
	repo.IndexerStatus.CommitSha = sha
	_, err := x.ID(repo.IndexerStatus.ID).Cols("commit_sha").
		Update(repo.IndexerStatus)
	return err
}
