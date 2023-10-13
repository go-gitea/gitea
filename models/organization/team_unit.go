// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
)

// TeamUnit describes all units of a repository
type TeamUnit struct {
	ID         int64     `xorm:"pk autoincr"`
	OrgID      int64     `xorm:"INDEX"`
	TeamID     int64     `xorm:"UNIQUE(s)"`
	Type       unit.Type `xorm:"UNIQUE(s)"`
	AccessMode perm.AccessMode
}

// Unit returns Unit
func (t *TeamUnit) Unit() unit.Unit {
	return unit.Units[t.Type]
}

func getUnitsByTeamID(ctx context.Context, teamID int64) (units []*TeamUnit, err error) {
	return units, db.GetEngine(ctx).Where("team_id = ?", teamID).Find(&units)
}

// UpdateTeamUnits updates a teams's units
func UpdateTeamUnits(ctx context.Context, team *Team, units []TeamUnit) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err = db.GetEngine(ctx).Where("team_id = ?", team.ID).Delete(new(TeamUnit)); err != nil {
		return err
	}

	if len(units) > 0 {
		if err = db.Insert(ctx, units); err != nil {
			return err
		}
	}

	return committer.Commit()
}
