// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
)

// RepoArchiver represents all archivers
type RepoArchiver struct {
	ID          int64           `xorm:"pk autoincr"`
	RepoID      int64           `xorm:"index unique(s)"`
	Type        git.ArchiveType `xorm:"unique(s)"`
	CommitID    string          `xorm:"VARCHAR(40) unique(s)"`
	Name        string
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
}

// GetRepoArchiver get an archiver
func GetRepoArchiver(ctx DBContext, repoID int64, tp git.ArchiveType, commitID string) (*RepoArchiver, error) {
	var archiver RepoArchiver
	has, err := ctx.e.Where("repo_id=?", repoID).And("`type`=?", tp).And("commit_id=?", commitID).Get(&archiver)
	if err != nil {
		return nil, err
	}
	if has {
		return &archiver, nil
	}
	return nil, nil
}

// AddArchiver adds an archiver
func AddArchiver(ctx DBContext, archiver *RepoArchiver) error {
	_, err := ctx.e.Insert(archiver)
	return err
}
