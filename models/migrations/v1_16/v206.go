// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddAuthorizeColForTeamUnit(x *xorm.Engine) error {
	type TeamUnit struct {
		ID         int64 `xorm:"pk autoincr"`
		OrgID      int64 `xorm:"INDEX"`
		TeamID     int64 `xorm:"UNIQUE(s)"`
		Type       int   `xorm:"UNIQUE(s)"`
		AccessMode int
	}

	if err := x.Sync(new(TeamUnit)); err != nil {
		return fmt.Errorf("sync2: %w", err)
	}

	// migrate old permission
	_, err := x.Exec("UPDATE team_unit SET access_mode = (SELECT authorize FROM team WHERE team.id = team_unit.team_id)")
	return err
}
