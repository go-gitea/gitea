// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddBranchTable(x *xorm.Engine) error {
	type Branch struct {
		ID            int64
		RepoID        int64  `xorm:"UNIQUE(s)"`
		Name          string `xorm:"UNIQUE(s) NOT NULL"`
		CommitID      string
		CommitMessage string `xorm:"TEXT"`
		PusherID      int64
		IsDeleted     bool `xorm:"index"`
		DeletedByID   int64
		DeletedUnix   timeutil.TimeStamp `xorm:"index"`
		CommitTime    timeutil.TimeStamp // The commit
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}

	if err := x.Sync(new(Branch)); err != nil {
		return err
	}

	if exist, err := x.IsTableExist("deleted_branches"); err != nil {
		return err
	} else if !exist {
		return nil
	}

	type DeletedBranch struct {
		ID          int64
		RepoID      int64  `xorm:"index UNIQUE(s)"`
		Name        string `xorm:"UNIQUE(s) NOT NULL"`
		Commit      string
		DeletedByID int64
		DeletedUnix timeutil.TimeStamp
	}

	var adminUserID int64
	has, err := x.Table("user").
		Select("id").
		Where("is_admin=?", true).
		Asc("id"). // Reliably get the admin with the lowest ID.
		Get(&adminUserID)
	if err != nil {
		return err
	} else if !has {
		return fmt.Errorf("no admin user found")
	}

	branches := make([]Branch, 0, 100)
	if err := db.Iterate(context.Background(), nil, func(ctx context.Context, deletedBranch *DeletedBranch) error {
		branches = append(branches, Branch{
			RepoID:      deletedBranch.RepoID,
			Name:        deletedBranch.Name,
			CommitID:    deletedBranch.Commit,
			PusherID:    adminUserID,
			IsDeleted:   true,
			DeletedByID: deletedBranch.DeletedByID,
			DeletedUnix: deletedBranch.DeletedUnix,
		})
		if len(branches) >= 100 {
			_, err := x.Insert(&branches)
			if err != nil {
				return err
			}
			branches = branches[:0]
		}
		return nil
	}); err != nil {
		return err
	}

	if len(branches) > 0 {
		if _, err := x.Insert(&branches); err != nil {
			return err
		}
	}

	return x.DropTables(new(DeletedBranch))
}
