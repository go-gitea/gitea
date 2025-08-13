package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
)

// GroupUnit describes all units of a repository group
type GroupUnit struct {
	ID         int64     `xorm:"pk autoincr"`
	GroupID    int64     `xorm:"UNIQUE(s)"`
	TeamID     int64     `xorm:"UNIQUE(s)"`
	Type       unit.Type `xorm:"UNIQUE(s)"`
	AccessMode perm.AccessMode
}

func (g *GroupUnit) Unit() unit.Unit {
	return unit.Units[g.Type]
}

func GetUnitsByGroupID(ctx context.Context, groupID int64) (units []*GroupUnit, err error) {
	return units, db.GetEngine(ctx).Where("group_id = ?", groupID).Find(&units)
}

func GetGroupUnit(ctx context.Context, groupID, teamID int64, unitType unit.Type) (unit *GroupUnit, err error) {
	unit = new(GroupUnit)
	_, err = db.GetEngine(ctx).
		Where("group_id = ?", groupID).
		And("team_id = ?", teamID).
		And("type = ?", unitType).
		Get(unit)
	return
}

func GetMaxGroupUnit(ctx context.Context, groupID int64, unitType unit.Type) (unit *GroupUnit, err error) {
	units := make([]*GroupUnit, 0)
	err = db.GetEngine(ctx).
		Where("group_id = ?", groupID).
		And("type = ?", unitType).
		Find(&units)
	if err != nil {
		return
	}
	for _, u := range units {
		if unit == nil || u.AccessMode > unit.AccessMode {
			unit = u
		}
	}
	return
}
