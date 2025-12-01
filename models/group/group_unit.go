// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
)

// RepoGroupUnit describes all units of a repository group
type RepoGroupUnit struct {
	ID         int64     `xorm:"pk autoincr"`
	GroupID    int64     `xorm:"UNIQUE(s)"`
	TeamID     int64     `xorm:"UNIQUE(s)"`
	Type       unit.Type `xorm:"UNIQUE(s)"`
	AccessMode perm.AccessMode
}

func (g *RepoGroupUnit) Unit() unit.Unit {
	return unit.Units[g.Type]
}

func GetUnitsByGroupID(ctx context.Context, groupID, teamID int64) (units []*RepoGroupUnit, err error) {
	return units, db.GetEngine(ctx).Where("group_id = ?", groupID).And("team_id = ?", teamID).Find(&units)
}

func GetGroupUnit(ctx context.Context, groupID, teamID int64, unitType unit.Type) (unit *RepoGroupUnit, err error) {
	unit = new(RepoGroupUnit)
	_, err = db.GetEngine(ctx).
		Where("group_id = ?", groupID).
		And("team_id = ?", teamID).
		And("type = ?", unitType).
		Get(unit)
	return unit, err
}

func GetMaxGroupUnit(ctx context.Context, groupID int64, unitType unit.Type) (unit *RepoGroupUnit, err error) {
	units := make([]*RepoGroupUnit, 0)
	err = db.GetEngine(ctx).
		Where("group_id = ?", groupID).
		And("type = ?", unitType).
		Find(&units)
	if err != nil {
		return nil, err
	}
	for _, u := range units {
		if unit == nil || u.AccessMode > unit.AccessMode {
			unit = u
		}
	}
	return unit, err
}
