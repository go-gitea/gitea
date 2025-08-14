package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
)

func GetTeamsWithAccessToGroup(ctx context.Context, orgID, groupID int64, mode perm.AccessMode) ([]*Team, error) {
	teams := make([]*Team, 0)
	inCond := group_model.ParentGroupCond(ctx, "repo_group_team.group_id", groupID)
	return teams, db.GetEngine(ctx).Distinct("team.*").Where("repo_group_team.access_mode >= ?", mode).
		Join("INNER", "repo_group_team", "repo_group_team.team_id = team.id and repo_group_team.org_id = ?", orgID).
		And("repo_group_team.org_id = ?", orgID).
		And(inCond).
		OrderBy("name").
		Find(&teams)
}

func GetTeamsWithAccessToGroupUnit(ctx context.Context, orgID, groupID int64, mode perm.AccessMode, unitType unit.Type) ([]*Team, error) {
	teams := make([]*Team, 0)
	inCond := group_model.ParentGroupCond(ctx, "repo_group_team.group_id", groupID)
	return teams, db.GetEngine(ctx).Where("repo_group_team.access_mode >= ?", mode).
		Join("INNER", "repo_group_team", "repo_group_team.team_id = team.id").
		Join("INNER", "repo_group_unit", "repo_group_unit.team_id = team.id").
		And("repo_group_team.org_id = ?", orgID).
		And(inCond).
		And("repo_group_unit.type = ?", unitType).
		OrderBy("name").
		Find(&teams)
}
