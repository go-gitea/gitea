package group

import (
	"code.gitea.io/gitea/models/db"
	"context"
)

// GroupTeam represents a relation for a team's access to a group
type GroupTeam struct {
	ID      int64 `xorm:"pk autoincr"`
	OrgID   int64 `xorm:"INDEX"`
	TeamID  int64 `xorm:"UNIQUE(s)"`
	GroupID int64 `xorm:"UNIQUE(s)"`
}

// HasTeamGroup returns true if the given group belongs to team.
func HasTeamGroup(ctx context.Context, orgID, teamID, groupID int64) bool {
	has, _ := db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("group_id=?", groupID).
		Get(new(GroupTeam))
	return has
}

func AddTeamGroup(ctx context.Context, orgID, teamID, groupID int64) error {
	_, err := db.GetEngine(ctx).Insert(&GroupTeam{
		OrgID:   orgID,
		GroupID: groupID,
		TeamID:  teamID,
	})
	return err
}

func RemoveTeamGroup(ctx context.Context, orgID, teamID, groupID int64) error {
	_, err := db.DeleteByBean(ctx, &GroupTeam{
		TeamID:  teamID,
		GroupID: groupID,
		OrgID:   orgID,
	})
	return err
}
