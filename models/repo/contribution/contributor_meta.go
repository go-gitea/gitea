// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package contribution

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// ContributorMeta stores metadata for contributor stats updates.
type ContributorMeta struct {
	RepoID                int64              `xorm:"pk"`
	LastProcessedCommitID string             `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	Dirty                 bool               `xorm:"NOT NULL DEFAULT false"`
	UpdatedUnix           timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (ContributorMeta) TableName() string {
	return "repo_contributor_meta"
}

func init() {
	db.RegisterModel(new(ContributorMeta))
}

// GetRepoContributorMeta returns repo contributor metadata if it exists.
func GetRepoContributorMeta(ctx context.Context, repoID int64) (*ContributorMeta, bool, error) {
	meta := &ContributorMeta{}
	has, err := db.GetEngine(ctx).ID(repoID).Get(meta)
	if err != nil {
		return nil, false, err
	}
	return meta, has, nil
}

// EnsureRepoContributorMeta ensures metadata row exists.
func EnsureRepoContributorMeta(ctx context.Context, repoID int64) (*ContributorMeta, error) {
	meta, has, err := GetRepoContributorMeta(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if has {
		return meta, nil
	}
	meta = &ContributorMeta{RepoID: repoID, UpdatedUnix: timeutil.TimeStampNow()}
	if _, err := db.GetEngine(ctx).Insert(meta); err != nil {
		meta, _, getErr := GetRepoContributorMeta(ctx, repoID)
		if getErr != nil {
			return nil, getErr
		}
		return meta, err
	}
	return meta, nil
}

// UpdateRepoContributorMeta updates specified columns for contributor metadata.
func UpdateRepoContributorMeta(ctx context.Context, meta *ContributorMeta, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(meta.RepoID).Cols(cols...).Update(meta)
	return err
}
