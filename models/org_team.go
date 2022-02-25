// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

const ownerTeamName = "Owners"

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
}

// SearchOrgTeamOptions holds the search options
type SearchOrgTeamOptions struct {
	db.ListOptions
	Keyword     string
	OrgID       int64
	IncludeDesc bool
}

// GetUserTeamOptions holds the search options.
type GetUserTeamOptions struct {
	db.ListOptions
	UserID int64
}

// SearchMembersOptions holds the search options
type SearchMembersOptions struct {
	db.ListOptions
}

// GetUserTeams search for org teams. Caller is responsible to check permissions.
func GetUserTeams(opts *GetUserTeamOptions) ([]*Team, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize == 0 {
		// Default limit
		opts.PageSize = 10
	}

	sess := db.GetEngine(db.DefaultContext)

	sess = sess.Join("INNER", "team_user", "team_user.team_id = team.id").
		And("team_user.uid=?", opts.UserID)

	count, err := sess.
		Count(new(Team))
	if err != nil {
		return nil, 0, err
	}

	if opts.PageSize == -1 {
		opts.PageSize = int(count)
	} else {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}

	sess = sess.Join("INNER", "team_user", "team_user.team_id = team.id").
		And("team_user.uid=?", opts.UserID)

	teams := make([]*Team, 0, opts.PageSize)
	if err = sess.
		OrderBy("lower_name").
		Find(&teams); err != nil {
		return nil, 0, err
	}

	return teams, count, nil
}

// SearchOrgTeams search for org teams. Caller is responsible to check permissions.
func SearchOrgTeams(opts *SearchOrgTeamOptions) ([]*Team, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize == 0 {
		// Default limit
		opts.PageSize = 10
	}

	cond := builder.NewCond()

	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		var keywordCond builder.Cond = builder.Like{"lower_name", lowerKeyword}
		if opts.IncludeDesc {
			keywordCond = keywordCond.Or(builder.Like{"LOWER(description)", lowerKeyword})
		}
		cond = cond.And(keywordCond)
	}

	cond = cond.And(builder.Eq{"org_id": opts.OrgID})

	sess := db.GetEngine(db.DefaultContext)

	count, err := sess.
		Where(cond).
		Count(new(Team))
	if err != nil {
		return nil, 0, err
	}

	sess = sess.Where(cond)
	if opts.PageSize == -1 {
		opts.PageSize = int(count)
	} else {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}

	teams := make([]*Team, 0, opts.PageSize)
	if err = sess.
		OrderBy("lower_name").
		Find(&teams); err != nil {
		return nil, 0, err
	}

	return teams, count, nil
}

// ColorFormat provides a basic color format for a Team
func (t *Team) ColorFormat(s fmt.State) {
	if t == nil {
		log.ColorFprintf(s, "%d:%s (OrgID: %d) %-v",
			log.NewColoredIDValue(0),
			"<nil>",
			log.NewColoredIDValue(0),
			0)
		return
	}
	log.ColorFprintf(s, "%d:%s (OrgID: %d) %-v",
		log.NewColoredIDValue(t.ID),
		t.Name,
		log.NewColoredIDValue(t.OrgID),
		t.AccessMode)
}

// GetUnits return a list of available units for a team
func (t *Team) GetUnits() error {
	return t.getUnits(db.GetEngine(db.DefaultContext))
}

func (t *Team) getUnits(e db.Engine) (err error) {
	if t.Units != nil {
		return nil
	}

	t.Units, err = getUnitsByTeamID(e, t.ID)
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
	return
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
	return t.Name == ownerTeamName
}

// IsMember returns true if given user is a member of team.
func (t *Team) IsMember(userID int64) bool {
	isMember, err := IsTeamMember(t.OrgID, t.ID, userID)
	if err != nil {
		log.Error("IsMember: %v", err)
		return false
	}
	return isMember
}

func (t *Team) getRepositories(e db.Engine) error {
	if t.Repos != nil {
		return nil
	}
	return e.Join("INNER", "team_repo", "repository.id = team_repo.repo_id").
		Where("team_repo.team_id=?", t.ID).
		OrderBy("repository.name").
		Find(&t.Repos)
}

// GetRepositories returns paginated repositories in team of organization.
func (t *Team) GetRepositories(opts *SearchOrgTeamOptions) error {
	if opts.Page == 0 {
		return t.getRepositories(db.GetEngine(db.DefaultContext))
	}

	return t.getRepositories(db.GetPaginatedSession(opts))
}

func (t *Team) getMembers(e db.Engine) (err error) {
	t.Members, err = getTeamMembers(e, t.ID)
	return err
}

// GetMembers returns paginated members in team of organization.
func (t *Team) GetMembers(opts *SearchMembersOptions) (err error) {
	if opts.Page == 0 {
		return t.getMembers(db.GetEngine(db.DefaultContext))
	}

	return t.getMembers(db.GetPaginatedSession(opts))
}

// AddMember adds new membership of the team to the organization,
// the user will have membership to the organization automatically when needed.
func (t *Team) AddMember(userID int64) error {
	return AddTeamMember(t, userID)
}

// RemoveMember removes member from team of organization.
func (t *Team) RemoveMember(userID int64) error {
	return RemoveTeamMember(t, userID)
}

func (t *Team) hasRepository(e db.Engine, repoID int64) bool {
	return hasTeamRepo(e, t.OrgID, t.ID, repoID)
}

// HasRepository returns true if given repository belong to team.
func (t *Team) HasRepository(repoID int64) bool {
	return t.hasRepository(db.GetEngine(db.DefaultContext), repoID)
}

func (t *Team) addRepository(ctx context.Context, repo *repo_model.Repository) (err error) {
	e := db.GetEngine(ctx)
	if err = addTeamRepo(e, t.OrgID, t.ID, repo.ID); err != nil {
		return err
	}

	if _, err = e.Incr("num_repos").ID(t.ID).Update(new(Team)); err != nil {
		return fmt.Errorf("update team: %v", err)
	}

	t.NumRepos++

	if err = recalculateTeamAccesses(ctx, repo, 0); err != nil {
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Make all team members watch this repo if enabled in global settings
	if setting.Service.AutoWatchNewRepos {
		if err = t.getMembers(e); err != nil {
			return fmt.Errorf("getMembers: %v", err)
		}
		for _, u := range t.Members {
			if err = repo_model.WatchRepoCtx(ctx, u.ID, repo.ID, true); err != nil {
				return fmt.Errorf("watchRepo: %v", err)
			}
		}
	}

	return nil
}

// addAllRepositories adds all repositories to the team.
// If the team already has some repositories they will be left unchanged.
func (t *Team) addAllRepositories(ctx context.Context) error {
	var orgRepos []repo_model.Repository
	e := db.GetEngine(ctx)
	if err := e.Where("owner_id = ?", t.OrgID).Find(&orgRepos); err != nil {
		return fmt.Errorf("get org repos: %v", err)
	}

	for _, repo := range orgRepos {
		if !t.hasRepository(e, repo.ID) {
			if err := t.addRepository(ctx, &repo); err != nil {
				return fmt.Errorf("addRepository: %v", err)
			}
		}
	}

	return nil
}

// AddAllRepositories adds all repositories to the team
func (t *Team) AddAllRepositories() (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = t.addAllRepositories(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// AddRepository adds new repository to team of organization.
func (t *Team) AddRepository(repo *repo_model.Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("Repository does not belong to organization")
	} else if t.HasRepository(repo.ID) {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = t.addRepository(ctx, repo); err != nil {
		return err
	}

	return committer.Commit()
}

// RemoveAllRepositories removes all repositories from team and recalculates access
func (t *Team) RemoveAllRepositories() (err error) {
	if t.IncludesAllRepositories {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = t.removeAllRepositories(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// removeAllRepositories removes all repositories from team and recalculates access
// Note: Shall not be called if team includes all repositories
func (t *Team) removeAllRepositories(ctx context.Context) (err error) {
	e := db.GetEngine(ctx)
	// Delete all accesses.
	for _, repo := range t.Repos {
		if err := recalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}

		// Remove watches from all users and now unaccessible repos
		for _, user := range t.Members {
			has, err := hasAccess(ctx, user.ID, repo)
			if err != nil {
				return err
			} else if has {
				continue
			}

			if err = repo_model.WatchRepoCtx(ctx, user.ID, repo.ID, false); err != nil {
				return err
			}

			// Remove all IssueWatches a user has subscribed to in the repositories
			if err = removeIssueWatchersByRepoID(e, user.ID, repo.ID); err != nil {
				return err
			}
		}
	}

	// Delete team-repo
	if _, err := e.
		Where("team_id=?", t.ID).
		Delete(new(TeamRepo)); err != nil {
		return err
	}

	t.NumRepos = 0
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	return nil
}

// removeRepository removes a repository from a team and recalculates access
// Note: Repository shall not be removed from team if it includes all repositories (unless the repository is deleted)
func (t *Team) removeRepository(ctx context.Context, repo *repo_model.Repository, recalculate bool) (err error) {
	e := db.GetEngine(ctx)
	if err = removeTeamRepo(e, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos--
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	// Don't need to recalculate when delete a repository from organization.
	if recalculate {
		if err = recalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}
	}

	teamUsers, err := getTeamUsersByTeamID(e, t.ID)
	if err != nil {
		return fmt.Errorf("getTeamUsersByTeamID: %v", err)
	}
	for _, teamUser := range teamUsers {
		has, err := hasAccess(ctx, teamUser.UID, repo)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = repo_model.WatchRepoCtx(ctx, teamUser.UID, repo.ID, false); err != nil {
			return err
		}

		// Remove all IssueWatches a user has subscribed to in the repositories
		if err := removeIssueWatchersByRepoID(e, teamUser.UID, repo.ID); err != nil {
			return err
		}
	}

	return nil
}

// RemoveRepository removes repository from team of organization.
// If the team shall include all repositories the request is ignored.
func (t *Team) RemoveRepository(repoID int64) error {
	if !t.HasRepository(repoID) {
		return nil
	}

	if t.IncludesAllRepositories {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(repoID)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = t.removeRepository(ctx, repo, true); err != nil {
		return err
	}

	return committer.Commit()
}

// UnitEnabled returns if the team has the given unit type enabled
func (t *Team) UnitEnabled(tp unit.Type) bool {
	return t.unitEnabled(db.GetEngine(db.DefaultContext), tp)
}

func (t *Team) unitEnabled(e db.Engine, tp unit.Type) bool {
	return t.unitAccessMode(e, tp) > perm.AccessModeNone
}

// UnitAccessMode returns if the team has the given unit type enabled
func (t *Team) UnitAccessMode(tp unit.Type) perm.AccessMode {
	return t.unitAccessMode(db.GetEngine(db.DefaultContext), tp)
}

func (t *Team) unitAccessMode(e db.Engine, tp unit.Type) perm.AccessMode {
	if err := t.getUnits(e); err != nil {
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

// NewTeam creates a record of new team.
// It's caller's responsibility to assign organization ID.
func NewTeam(t *Team) (err error) {
	if len(t.Name) == 0 {
		return errors.New("empty team name")
	}

	if err = IsUsableTeamName(t.Name); err != nil {
		return err
	}

	has, err := db.GetEngine(db.DefaultContext).ID(t.OrgID).Get(new(user_model.User))
	if err != nil {
		return err
	}
	if !has {
		return ErrOrgNotExist{t.OrgID, ""}
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err = db.GetEngine(db.DefaultContext).
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		Get(new(Team))
	if err != nil {
		return err
	}
	if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = db.Insert(ctx, t); err != nil {
		return err
	}

	// insert units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		if err = db.Insert(ctx, &t.Units); err != nil {
			return err
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if t.IncludesAllRepositories {
		err = t.addAllRepositories(ctx)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %v", err)
		}
	}

	// Update organization number of teams.
	if _, err = db.Exec(ctx, "UPDATE `user` SET num_teams=num_teams+1 WHERE id = ?", t.OrgID); err != nil {
		return err
	}
	return committer.Commit()
}

func getTeam(e db.Engine, orgID int64, name string) (*Team, error) {
	t := &Team{
		OrgID:     orgID,
		LowerName: strings.ToLower(name),
	}
	has, err := e.Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{orgID, 0, name}
	}
	return t, nil
}

// GetTeam returns team by given team name and organization.
func GetTeam(orgID int64, name string) (*Team, error) {
	return getTeam(db.GetEngine(db.DefaultContext), orgID, name)
}

// GetTeamIDsByNames returns a slice of team ids corresponds to names.
func GetTeamIDsByNames(orgID int64, names []string, ignoreNonExistent bool) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		u, err := GetTeam(orgID, name)
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

// getOwnerTeam returns team by given team name and organization.
func getOwnerTeam(e db.Engine, orgID int64) (*Team, error) {
	return getTeam(e, orgID, ownerTeamName)
}

func getTeamByID(e db.Engine, teamID int64) (*Team, error) {
	t := new(Team)
	has, err := e.ID(teamID).Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{0, teamID, ""}
	}
	return t, nil
}

// GetTeamByID returns team by given ID.
func GetTeamByID(teamID int64) (*Team, error) {
	return getTeamByID(db.GetEngine(db.DefaultContext), teamID)
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

// UpdateTeam updates information of team.
func UpdateTeam(t *Team, authChanged, includeAllChanged bool) (err error) {
	if len(t.Name) == 0 {
		return errors.New("empty team name")
	}

	if len(t.Description) > 255 {
		t.Description = t.Description[:255]
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	t.LowerName = strings.ToLower(t.Name)
	has, err := sess.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		And("id!=?", t.ID).
		Get(new(Team))
	if err != nil {
		return err
	} else if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	if _, err = sess.ID(t.ID).Cols("name", "lower_name", "description",
		"can_create_org_repo", "authorize", "includes_all_repositories").Update(t); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	// update units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		// Delete team-unit.
		if _, err := sess.
			Where("team_id=?", t.ID).
			Delete(new(TeamUnit)); err != nil {
			return err
		}
		if _, err = sess.Cols("org_id", "team_id", "type", "access_mode").Insert(&t.Units); err != nil {
			return err
		}
	}

	// Update access for team members if needed.
	if authChanged {
		if err = t.getRepositories(sess); err != nil {
			return fmt.Errorf("getRepositories: %v", err)
		}

		for _, repo := range t.Repos {
			if err = recalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if includeAllChanged && t.IncludesAllRepositories {
		err = t.addAllRepositories(ctx)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %v", err)
		}
	}

	return committer.Commit()
}

// DeleteTeam deletes given team.
// It's caller's responsibility to assign organization ID.
func DeleteTeam(t *Team) error {
	if err := t.GetRepositories(&SearchOrgTeamOptions{}); err != nil {
		return err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	if err := t.getMembers(sess); err != nil {
		return err
	}

	if err := t.removeAllRepositories(ctx); err != nil {
		return err
	}

	// Delete team-user.
	if _, err := sess.
		Where("org_id=?", t.OrgID).
		Where("team_id=?", t.ID).
		Delete(new(TeamUser)); err != nil {
		return err
	}

	// Delete team-unit.
	if _, err := sess.
		Where("team_id=?", t.ID).
		Delete(new(TeamUnit)); err != nil {
		return err
	}

	// Delete team.
	if _, err := sess.ID(t.ID).Delete(new(Team)); err != nil {
		return err
	}
	// Update organization number of teams.
	if _, err := sess.Exec("UPDATE `user` SET num_teams=num_teams-1 WHERE id=?", t.OrgID); err != nil {
		return err
	}

	return committer.Commit()
}

// ___________                    ____ ___
// \__    ___/___ _____    _____ |    |   \______ ___________
//   |    |_/ __ \\__  \  /     \|    |   /  ___// __ \_  __ \
//   |    |\  ___/ / __ \|  Y Y  \    |  /\___ \\  ___/|  | \/
//   |____| \___  >____  /__|_|  /______//____  >\___  >__|
//              \/     \/      \/             \/     \/

// TeamUser represents an team-user relation.
type TeamUser struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	UID    int64 `xorm:"UNIQUE(s)"`
}

func isTeamMember(e db.Engine, orgID, teamID, userID int64) (bool, error) {
	return e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("uid=?", userID).
		Table("team_user").
		Exist()
}

// IsTeamMember returns true if given user is a member of team.
func IsTeamMember(orgID, teamID, userID int64) (bool, error) {
	return isTeamMember(db.GetEngine(db.DefaultContext), orgID, teamID, userID)
}

func getTeamUsersByTeamID(e db.Engine, teamID int64) ([]*TeamUser, error) {
	teamUsers := make([]*TeamUser, 0, 10)
	return teamUsers, e.
		Where("team_id=?", teamID).
		Find(&teamUsers)
}

func getTeamMembers(e db.Engine, teamID int64) (_ []*user_model.User, err error) {
	teamUsers, err := getTeamUsersByTeamID(e, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team-users: %v", err)
	}
	members := make([]*user_model.User, len(teamUsers))
	for i, teamUser := range teamUsers {
		member, err := user_model.GetUserByIDEngine(e, teamUser.UID)
		if err != nil {
			return nil, fmt.Errorf("get user '%d': %v", teamUser.UID, err)
		}
		members[i] = member
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].DisplayName() < members[j].DisplayName()
	})
	return members, nil
}

// GetTeamMembers returns all members in given team of organization.
func GetTeamMembers(teamID int64) ([]*user_model.User, error) {
	return getTeamMembers(db.GetEngine(db.DefaultContext), teamID)
}

func getUserOrgTeams(e db.Engine, orgID, userID int64) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid=?", userID).
		Find(&teams)
}

func getUserRepoTeams(e db.Engine, orgID, userID, repoID int64) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid=?", userID).
		And("team_repo.repo_id=?", repoID).
		Find(&teams)
}

// GetUserOrgTeams returns all teams that user belongs to in given organization.
func GetUserOrgTeams(orgID, userID int64) ([]*Team, error) {
	return getUserOrgTeams(db.GetEngine(db.DefaultContext), orgID, userID)
}

// AddTeamMember adds new membership of given team to given organization,
// the user will have membership to given organization automatically when needed.
func AddTeamMember(team *Team, userID int64) error {
	isAlreadyMember, err := IsTeamMember(team.OrgID, team.ID, userID)
	if err != nil || isAlreadyMember {
		return err
	}

	if err := AddOrgUser(team.OrgID, userID); err != nil {
		return err
	}

	// Get team and its repositories.
	if err := team.GetRepositories(&SearchOrgTeamOptions{}); err != nil {
		return err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	if err := db.Insert(ctx, &TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err := sess.Incr("num_members").ID(team.ID).Update(new(Team)); err != nil {
		return err
	}

	team.NumMembers++

	// Give access to team repositories.
	for _, repo := range team.Repos {
		if err := recalculateUserAccess(ctx, repo, userID); err != nil {
			return err
		}
		if setting.Service.AutoWatchNewRepos {
			if err = repo_model.WatchRepoCtx(ctx, userID, repo.ID, true); err != nil {
				return err
			}
		}
	}

	return committer.Commit()
}

func removeTeamMember(ctx context.Context, team *Team, userID int64) error {
	e := db.GetEngine(ctx)
	isMember, err := isTeamMember(e, team.OrgID, team.ID, userID)
	if err != nil || !isMember {
		return err
	}

	// Check if the user to delete is the last member in owner team.
	if team.IsOwnerTeam() && team.NumMembers == 1 {
		return ErrLastOrgOwner{UID: userID}
	}

	team.NumMembers--

	if err := team.getRepositories(e); err != nil {
		return err
	}

	if _, err := e.Delete(&TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err = e.
		ID(team.ID).
		Cols("num_members").
		Update(team); err != nil {
		return err
	}

	// Delete access to team repositories.
	for _, repo := range team.Repos {
		if err := recalculateUserAccess(ctx, repo, userID); err != nil {
			return err
		}

		// Remove watches from now unaccessible
		if err := reconsiderWatches(ctx, repo, userID); err != nil {
			return err
		}

		// Remove issue assignments from now unaccessible
		if err := reconsiderRepoIssuesAssignee(ctx, repo, userID); err != nil {
			return err
		}
	}

	// Check if the user is a member of any team in the organization.
	if count, err := e.Count(&TeamUser{
		UID:   userID,
		OrgID: team.OrgID,
	}); err != nil {
		return err
	} else if count == 0 {
		return removeOrgUser(ctx, team.OrgID, userID)
	}

	return nil
}

// RemoveTeamMember removes member from given team of given organization.
func RemoveTeamMember(team *Team, userID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	if err := removeTeamMember(ctx, team, userID); err != nil {
		return err
	}
	return committer.Commit()
}

// IsUserInTeams returns if a user in some teams
func IsUserInTeams(userID int64, teamIDs []int64) (bool, error) {
	return isUserInTeams(db.GetEngine(db.DefaultContext), userID, teamIDs)
}

func isUserInTeams(e db.Engine, userID int64, teamIDs []int64) (bool, error) {
	return e.Where("uid=?", userID).In("team_id", teamIDs).Exist(new(TeamUser))
}

// UsersInTeamsCount counts the number of users which are in userIDs and teamIDs
func UsersInTeamsCount(userIDs, teamIDs []int64) (int64, error) {
	var ids []int64
	if err := db.GetEngine(db.DefaultContext).In("uid", userIDs).In("team_id", teamIDs).
		Table("team_user").
		Cols("uid").GroupBy("uid").Find(&ids); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// ___________                  __________
// \__    ___/___ _____    _____\______   \ ____ ______   ____
//   |    |_/ __ \\__  \  /     \|       _// __ \\____ \ /  _ \
//   |    |\  ___/ / __ \|  Y Y  \    |   \  ___/|  |_> >  <_> )
//   |____| \___  >____  /__|_|  /____|_  /\___  >   __/ \____/
//              \/     \/      \/       \/     \/|__|

// TeamRepo represents an team-repository relation.
type TeamRepo struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
}

func hasTeamRepo(e db.Engine, orgID, teamID, repoID int64) bool {
	has, _ := e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("repo_id=?", repoID).
		Get(new(TeamRepo))
	return has
}

// HasTeamRepo returns true if given repository belongs to team.
func HasTeamRepo(orgID, teamID, repoID int64) bool {
	return hasTeamRepo(db.GetEngine(db.DefaultContext), orgID, teamID, repoID)
}

func addTeamRepo(e db.Engine, orgID, teamID, repoID int64) error {
	_, err := e.InsertOne(&TeamRepo{
		OrgID:  orgID,
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

func removeTeamRepo(e db.Engine, teamID, repoID int64) error {
	_, err := e.Delete(&TeamRepo{
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// GetTeamsWithAccessToRepo returns all teams in an organization that have given access level to the repository.
func GetTeamsWithAccessToRepo(orgID, repoID int64, mode perm.AccessMode) ([]*Team, error) {
	teams := make([]*Team, 0, 5)
	return teams, db.GetEngine(db.DefaultContext).Where("team.authorize >= ?", mode).
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		And("team_repo.org_id = ?", orgID).
		And("team_repo.repo_id = ?", repoID).
		Find(&teams)
}

// ___________                    ____ ___      .__  __
// \__    ___/___ _____    _____ |    |   \____ |__|/  |_
//   |    |_/ __ \\__  \  /     \|    |   /    \|  \   __\
//   |    |\  ___/ / __ \|  Y Y  \    |  /   |  \  ||  |
//   |____| \___  >____  /__|_|  /______/|___|  /__||__|
//              \/     \/      \/             \/

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

func getUnitsByTeamID(e db.Engine, teamID int64) (units []*TeamUnit, err error) {
	return units, e.Where("team_id = ?", teamID).Find(&units)
}

// UpdateTeamUnits updates a teams's units
func UpdateTeamUnits(team *Team, units []TeamUnit) (err error) {
	ctx, committer, err := db.TxContext()
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
