package organization

import (
	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"context"
)

func GetTeamsWithAccessToGroup(ctx context.Context, orgID, groupID int64, mode perm.AccessMode) ([]*Team, error) {
	teams := make([]*Team, 0)
	inCond := group_model.ParentGroupCond(ctx, "group_team.group_id", groupID)
	return teams, db.GetEngine(ctx).Distinct("team.*").Where("group_team.access_mode >= ?", mode).
		Join("INNER", "group_team", "group_team.team_id = team.id and group_team.org_id = ?", orgID).
		And("group_team.org_id = ?", orgID).
		And(inCond).
		OrderBy("name").
		Find(&teams)
}

func GetTeamsWithAccessToGroupUnit(ctx context.Context, orgID, groupID int64, mode perm.AccessMode, unitType unit.Type) ([]*Team, error) {
	teams := make([]*Team, 0)
	inCond := group_model.ParentGroupCond(ctx, "group_team.group_id", groupID)
	return teams, db.GetEngine(ctx).Where("group_team.access_mode >= ?", mode).
		Join("INNER", "group_team", "group_team.team_id = team.id").
		Join("INNER", "group_unit", "group_unit.team_id = team.id").
		And("group_team.org_id = ?", orgID).
		And(inCond).
		And("group_unit.type = ?", unitType).
		OrderBy("name").
		Find(&teams)
}
