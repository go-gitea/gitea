package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// GroupTeam represents a relation for a team's access to a group
type GroupTeam struct {
	ID          int64 `xorm:"pk autoincr"`
	OrgID       int64 `xorm:"INDEX"`
	TeamID      int64 `xorm:"UNIQUE(s)"`
	GroupID     int64 `xorm:"UNIQUE(s)"`
	AccessMode  perm.AccessMode
	CanCreateIn bool
	Units       []*GroupUnit `xorm:"-"`
}

func (g *GroupTeam) LoadGroupUnits(ctx context.Context) (err error) {
	g.Units, err = GetUnitsByGroupID(ctx, g.GroupID)
	return
}

func (g *GroupTeam) UnitAccessModeEx(ctx context.Context, tp unit.Type) (accessMode perm.AccessMode, exist bool) {
	accessMode = perm.AccessModeNone
	if err := g.LoadGroupUnits(ctx); err != nil {
		log.Warn("Error loading units of team for group[%d] (ID: %d): %s", g.GroupID, g.TeamID, err.Error())
	}
	for _, u := range g.Units {
		if u.Type == tp {
			accessMode = u.AccessMode
			exist = true
			break
		}
	}
	return
}

// HasTeamGroup returns true if the given group belongs to a team.
func HasTeamGroup(ctx context.Context, orgID, teamID, groupID int64) bool {
	has, _ := db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("group_id=?", groupID).
		And("access_mode >= ?", perm.AccessModeRead).
		Get(new(GroupTeam))
	return has
}

// AddTeamGroup adds a group to a team
func AddTeamGroup(ctx context.Context, orgID, teamID, groupID int64, access perm.AccessMode, canCreateIn bool) error {
	if access <= perm.AccessModeWrite {
		canCreateIn = false
	}
	_, err := db.GetEngine(ctx).Insert(&GroupTeam{
		OrgID:       orgID,
		GroupID:     groupID,
		TeamID:      teamID,
		AccessMode:  access,
		CanCreateIn: canCreateIn,
	})
	return err
}

func UpdateTeamGroup(ctx context.Context, orgID, teamID, groupID int64, access perm.AccessMode, canCreateIn, isNew bool) (err error) {
	if access <= perm.AccessModeNone {
		canCreateIn = false
	}
	if isNew {
		err = AddTeamGroup(ctx, orgID, teamID, groupID, access, canCreateIn)
	} else {
		_, err = db.GetEngine(ctx).
			Table("group_team").
			Where("org_id=?", orgID).
			And("team_id=?", teamID).
			And("group_id =?", groupID).
			Update(&GroupTeam{
				OrgID:       orgID,
				TeamID:      teamID,
				GroupID:     groupID,
				AccessMode:  access,
				CanCreateIn: canCreateIn,
			})
	}

	return err
}

// RemoveTeamGroup removes a group from a team
func RemoveTeamGroup(ctx context.Context, orgID, teamID, groupID int64) error {
	_, err := db.DeleteByBean(ctx, &GroupTeam{
		TeamID:  teamID,
		GroupID: groupID,
		OrgID:   orgID,
	})
	return err
}

func FindGroupTeams(ctx context.Context, groupID int64) (gteams []*GroupTeam, err error) {
	return gteams, db.GetEngine(ctx).
		Where("group_id=?", groupID).
		Table("group_team").
		Find(&gteams)
}

func FindGroupTeamByTeamID(ctx context.Context, groupID, teamID int64) (gteam *GroupTeam, err error) {
	gteam = new(GroupTeam)
	has, err := db.GetEngine(ctx).
		Where("group_id=?", groupID).
		And("team_id = ?", teamID).
		Table("group_team").
		Get(gteam)
	if !has {
		gteam = nil
	}
	return
}

func GetAncestorPermissions(ctx context.Context, groupID, teamID int64) (perm.AccessMode, error) {
	sess := db.GetEngine(ctx)
	groups, err := GetParentGroupIDChain(ctx, groupID)
	if err != nil {
		return perm.AccessModeNone, err
	}
	gteams := make([]*GroupTeam, 0)
	err = sess.In("group_id", groups).And("team_id = ?", teamID).Find(&gteams)
	if err != nil {
		return perm.AccessModeNone, err
	}
	mapped := util.SliceMap(gteams, func(g *GroupTeam) perm.AccessMode {
		return g.AccessMode
	})
	maxMode := max(mapped[0])

	for _, m := range mapped[1:] {
		maxMode = max(maxMode, m)
	}
	return maxMode, nil
}
