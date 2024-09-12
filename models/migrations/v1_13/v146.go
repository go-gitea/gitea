// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddProjectsInfo(x *xorm.Engine) error {
	// Create new tables
	type (
		ProjectType      uint8
		ProjectBoardType uint8
	)

	type Project struct {
		ID          int64  `xorm:"pk autoincr"`
		Title       string `xorm:"INDEX NOT NULL"`
		Description string `xorm:"TEXT"`
		RepoID      int64  `xorm:"INDEX"`
		CreatorID   int64  `xorm:"NOT NULL"`
		IsClosed    bool   `xorm:"INDEX"`

		BoardType ProjectBoardType
		Type      ProjectType

		ClosedDateUnix timeutil.TimeStamp
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync(new(Project)); err != nil {
		return err
	}

	type Comment struct {
		OldProjectID int64
		ProjectID    int64
	}

	if err := x.Sync(new(Comment)); err != nil {
		return err
	}

	type Repository struct {
		ID                int64
		NumProjects       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects int `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	// ProjectIssue saves relation from issue to a project
	type ProjectIssue struct {
		ID             int64 `xorm:"pk autoincr"`
		IssueID        int64 `xorm:"INDEX"`
		ProjectID      int64 `xorm:"INDEX"`
		ProjectBoardID int64 `xorm:"INDEX"`
	}

	if err := x.Sync(new(ProjectIssue)); err != nil {
		return err
	}

	type ProjectBoard struct {
		ID      int64 `xorm:"pk autoincr"`
		Title   string
		Default bool `xorm:"NOT NULL DEFAULT false"`

		ProjectID int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(ProjectBoard))
}
