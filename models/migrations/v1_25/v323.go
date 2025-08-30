// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type ActionDeployment struct {
	ID            int64  `xorm:"pk autoincr"`
	RepoID        int64  `xorm:"INDEX NOT NULL"`
	RunID         int64  `xorm:"INDEX NOT NULL"`
	EnvironmentID int64  `xorm:"INDEX NOT NULL"`
	Ref           string `xorm:"INDEX"`
	CommitSHA     string `xorm:"INDEX"`
	Task          string
	Status        string `xorm:"INDEX"`
	Description   string `xorm:"TEXT"`
	LogURL        string `xorm:"TEXT"`
	CreatedByID   int64  `xorm:"INDEX"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

func CreateActionDeploymentTable(x *xorm.Engine) error {
	return x.Sync(new(ActionDeployment))
}