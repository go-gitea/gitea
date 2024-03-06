// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddTeamInviteTable(x *xorm.Engine) error {
	type TeamInvite struct {
		ID          int64              `xorm:"pk autoincr"`
		Token       string             `xorm:"UNIQUE(token) INDEX NOT NULL DEFAULT ''"`
		InviterID   int64              `xorm:"NOT NULL DEFAULT 0"`
		OrgID       int64              `xorm:"INDEX NOT NULL DEFAULT 0"`
		TeamID      int64              `xorm:"UNIQUE(team_mail) INDEX NOT NULL DEFAULT 0"`
		Email       string             `xorm:"UNIQUE(team_mail) NOT NULL DEFAULT ''"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(TeamInvite))
}
