package group

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"context"
)

// GroupUnit describes all units of a repository group
type GroupUnit struct {
	ID         int64     `xorm:"pk autoincr"`
	GroupID    int64     `xorm:"INDEX"`
	TeamID     int64     `xorm:"UNIQUE(s)"`
	Type       unit.Type `xorm:"UNIQUE(s)"`
	AccessMode perm.AccessMode
}

func (g *GroupUnit) Unit() unit.Unit {
	return unit.Units[g.Type]
}

func getUnitsByGroupID(ctx context.Context, groupID int64) (units []*GroupUnit, err error) {
	return units, db.GetEngine(ctx).Where("group_id = ?", groupID).Find(&units)
}
