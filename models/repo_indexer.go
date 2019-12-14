// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// RepoIndexerStatus status of a repo's entry in the repo indexer
// For now, implicitly refers to default branch
type RepoIndexerStatus struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX"`
	CommitSha string `xorm:"VARCHAR(40)"`
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
