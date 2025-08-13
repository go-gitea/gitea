package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// RepoGroupTeam represents a relation for a team's access to a group
type RepoGroupTeam struct {
	ID          int64 `xorm:"pk autoincr"`
	OrgID       int64 `xorm:"INDEX"`
	TeamID      int64 `xorm:"UNIQUE(s)"`
	GroupID     int64 `xorm:"UNIQUE(s)"`
	AccessMode  perm.AccessMode
	CanCreateIn bool
	Units       []*RepoGroupUnit `xorm:"-"`
}

func (g *RepoGroupTeam) LoadGroupUnits(ctx context.Context) error {
	var err error
	g.Units, err = GetUnitsByGroupID(ctx, g.GroupID)
	return err
}

func (g *RepoGroupTeam) UnitAccessModeEx(ctx context.Context, tp unit.Type) (accessMode perm.AccessMode, exist bool) {
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
	return accessMode, exist
}

// HasTeamGroup returns true if the given group belongs to a team.
func HasTeamGroup(ctx context.Context, orgID, teamID, groupID int64) bool {
	has, _ := db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("group_id=?", groupID).
		And("access_mode >= ?", perm.AccessModeRead).
		Get(new(RepoGroupTeam))
	return has
}

// AddTeamGroup adds a group to a team
func AddTeamGroup(ctx context.Context, orgID, teamID, groupID int64, access perm.AccessMode, canCreateIn bool) error {
	if access <= perm.AccessModeWrite {
		canCreateIn = false
	}
	_, err := db.GetEngine(ctx).Insert(&RepoGroupTeam{
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
			Table("repo_group_team").
			Where("org_id=?", orgID).
			And("team_id=?", teamID).
			And("group_id =?", groupID).
			Update(&RepoGroupTeam{
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
	_, err := db.DeleteByBean(ctx, &RepoGroupTeam{
		TeamID:  teamID,
		GroupID: groupID,
		OrgID:   orgID,
	})
	return err
}

func FindGroupTeams(ctx context.Context, groupID int64) (gteams []*RepoGroupTeam, err error) {
	return gteams, db.GetEngine(ctx).
		Where("group_id=?", groupID).
		Table("repo_group_team").
		Find(&gteams)
}

func FindGroupTeamByTeamID(ctx context.Context, groupID, teamID int64) (gteam *RepoGroupTeam, err error) {
	gteam = new(RepoGroupTeam)
	has, err := db.GetEngine(ctx).
		Where("group_id=?", groupID).
		And("team_id = ?", teamID).
		Table("repo_group_team").
		Get(gteam)
	if !has {
		gteam = nil
	}
	return gteam, err
}

func GetAncestorPermissions(ctx context.Context, groupID, teamID int64) (perm.AccessMode, error) {
	sess := db.GetEngine(ctx)
	groups, err := GetParentGroupIDChain(ctx, groupID)
	if err != nil {
		return perm.AccessModeNone, err
	}
	gteams := make([]*RepoGroupTeam, 0)
	err = sess.In("group_id", groups).And("team_id = ?", teamID).Find(&gteams)
	if err != nil {
		return perm.AccessModeNone, err
	}
	mapped := util.SliceMap(gteams, func(g *RepoGroupTeam) perm.AccessMode {
		return g.AccessMode
	})
	maxMode := max(mapped[0])

	for _, m := range mapped[1:] {
		maxMode = max(maxMode, m)
	}
	return maxMode, nil
}
