// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// ___________
// \__    ___/___ _____    _____
//   |    |_/ __ \\__  \  /     \
//   |    |\  ___/ / __ \|  Y Y  \
//   |____| \___  >____  /__|_|  /
//              \/     \/      \/

// ErrTeamAlreadyExist represents a "TeamAlreadyExist" kind of error.
type ErrTeamAlreadyExist struct {
	OrgID int64
	Name  string
}

// IsErrTeamAlreadyExist checks if an error is a ErrTeamAlreadyExist.
func IsErrTeamAlreadyExist(err error) bool {
	_, ok := err.(ErrTeamAlreadyExist)
	return ok
}

func (err ErrTeamAlreadyExist) Error() string {
	return fmt.Sprintf("team already exists [org_id: %d, name: %s]", err.OrgID, err.Name)
}

func (err ErrTeamAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrTeamNotExist represents a "TeamNotExist" error
type ErrTeamNotExist struct {
	OrgID  int64
	TeamID int64
	Name   string
}

// IsErrTeamNotExist checks if an error is a ErrTeamNotExist.
func IsErrTeamNotExist(err error) bool {
	_, ok := err.(ErrTeamNotExist)
	return ok
}

func (err ErrTeamNotExist) Error() string {
	return fmt.Sprintf("team does not exist [org_id %d, team_id %d, name: %s]", err.OrgID, err.TeamID, err.Name)
}

func (err ErrTeamNotExist) Unwrap() error {
	return util.ErrNotExist
}

// OwnerTeamName return the owner team name
const OwnerTeamName = "Owners"

// Team represents a organization team.
type Team struct {
	ID                      int64 `xorm:"pk autoincr"`
	OrgID                   int64 `xorm:"INDEX"`
	LowerName               string
	Name                    string
	Description             string
	AccessMode              perm.AccessMode          `xorm:"'authorize'"`
	Repos                   []*repo_model.Repository `xorm:"-"`
	Members                 []*user_model.User       `xorm:"-"`
	NumRepos                int
	NumMembers              int
	Units                   []*TeamUnit `xorm:"-"`
	IncludesAllRepositories bool        `xorm:"NOT NULL DEFAULT false"`
	CanCreateOrgRepo        bool        `xorm:"NOT NULL DEFAULT false"`
}

func init() {
	db.RegisterModel(new(Team))
	db.RegisterModel(new(TeamUser))
	db.RegisterModel(new(TeamRepo))
	db.RegisterModel(new(TeamUnit))
	db.RegisterModel(new(TeamInvite))
}

func (t *Team) LogString() string {
	if t == nil {
		return "<Team nil>"
	}
	return fmt.Sprintf("<Team %d:%s OrgID=%d AccessMode=%s>", t.ID, t.Name, t.OrgID, t.AccessMode.LogString())
}

// LoadUnits load a list of available units for a team
func (t *Team) LoadUnits(ctx context.Context) (err error) {
	if t.Units != nil {
		return nil
	}

	t.Units, err = getUnitsByTeamID(ctx, t.ID)
	return err
}

// GetUnitNames returns the team units names
func (t *Team) GetUnitNames() (res []string) {
	if t.AccessMode >= perm.AccessModeAdmin {
		return unit.AllUnitKeyNames()
	}

	for _, u := range t.Units {
		res = append(res, unit.Units[u.Type].NameKey)
	}
	return res
}

// GetUnitsMap returns the team units permissions
func (t *Team) GetUnitsMap() map[string]string {
	m := make(map[string]string)
	if t.AccessMode >= perm.AccessModeAdmin {
		for _, u := range unit.Units {
			m[u.NameKey] = t.AccessMode.String()
		}
	} else {
		for _, u := range t.Units {
			m[u.Unit().NameKey] = u.AccessMode.String()
		}
	}
	return m
}

// IsOwnerTeam returns true if team is owner team.
func (t *Team) IsOwnerTeam() bool {
	return t.Name == OwnerTeamName
}

// IsMember returns true if given user is a member of team.
func (t *Team) IsMember(userID int64) bool {
	isMember, err := IsTeamMember(db.DefaultContext, t.OrgID, t.ID, userID)
	if err != nil {
		log.Error("IsMember: %v", err)
		return false
	}
	return isMember
}

// LoadRepositories returns paginated repositories in team of organization.
func (t *Team) LoadRepositories(ctx context.Context) (err error) {
	if t.Repos != nil {
		return nil
	}
	t.Repos, err = GetTeamRepositories(ctx, &SearchTeamRepoOptions{
		TeamID: t.ID,
	})
	return err
}

// LoadMembers returns paginated members in team of organization.
func (t *Team) LoadMembers(ctx context.Context) (err error) {
	t.Members, err = GetTeamMembers(ctx, &SearchMembersOptions{
		TeamID: t.ID,
	})
	return err
}

// UnitEnabled returns if the team has the given unit type enabled
func (t *Team) UnitEnabled(ctx context.Context, tp unit.Type) bool {
	return t.UnitAccessMode(ctx, tp) > perm.AccessModeNone
}

// UnitAccessMode returns if the team has the given unit type enabled
func (t *Team) UnitAccessMode(ctx context.Context, tp unit.Type) perm.AccessMode {
	if err := t.LoadUnits(ctx); err != nil {
		log.Warn("Error loading team (ID: %d) units: %s", t.ID, err.Error())
	}

	for _, unit := range t.Units {
		if unit.Type == tp {
			return unit.AccessMode
		}
	}
	return perm.AccessModeNone
}

// IsUsableTeamName tests if a name could be as team name
func IsUsableTeamName(name string) error {
	switch name {
	case "new":
		return db.ErrNameReserved{Name: name}
	default:
		return nil
	}
}

// GetTeam returns team by given team name and organization.
func GetTeam(ctx context.Context, orgID int64, name string) (*Team, error) {
	t := &Team{
		OrgID:     orgID,
		LowerName: strings.ToLower(name),
	}
	has, err := db.GetByBean(ctx, t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{orgID, 0, name}
	}
	return t, nil
}

// GetTeamIDsByNames returns a slice of team ids corresponds to names.
func GetTeamIDsByNames(orgID int64, names []string, ignoreNonExistent bool) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		u, err := GetTeam(db.DefaultContext, orgID, name)
		if err != nil {
			if ignoreNonExistent {
				continue
			} else {
				return nil, err
			}
		}
		ids = append(ids, u.ID)
	}
	return ids, nil
}

// GetOwnerTeam returns team by given team name and organization.
func GetOwnerTeam(ctx context.Context, orgID int64) (*Team, error) {
	return GetTeam(ctx, orgID, OwnerTeamName)
}

// GetTeamByID returns team by given ID.
func GetTeamByID(ctx context.Context, teamID int64) (*Team, error) {
	t := new(Team)
	has, err := db.GetEngine(ctx).ID(teamID).Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{0, teamID, ""}
	}
	return t, nil
}

// GetTeamNamesByID returns team's lower name from a list of team ids.
func GetTeamNamesByID(teamIDs []int64) ([]string, error) {
	if len(teamIDs) == 0 {
		return []string{}, nil
	}

	var teamNames []string
	err := db.GetEngine(db.DefaultContext).Table("team").
		Select("lower_name").
		In("id", teamIDs).
		Asc("name").
		Find(&teamNames)

	return teamNames, err
}

// IncrTeamRepoNum increases the number of repos for the given team by 1
func IncrTeamRepoNum(ctx context.Context, teamID int64) error {
	_, err := db.GetEngine(ctx).Incr("num_repos").ID(teamID).Update(new(Team))
	return err
}
