// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"strings"
)

const ownerTeamName = "Owners"

// Team represents a organization team.
type Team struct {
	ID          int64 `xorm:"pk autoincr"`
	OrgID       int64 `xorm:"INDEX"`
	LowerName   string
	Name        string
	Description string
	Authorize   AccessMode
	Repos       []*Repository `xorm:"-"`
	Members     []*User       `xorm:"-"`
	NumRepos    int
	NumMembers  int
}

// IsOwnerTeam returns true if team is owner team.
func (t *Team) IsOwnerTeam() bool {
	return t.Name == ownerTeamName
}

// IsMember returns true if given user is a member of team.
func (t *Team) IsMember(userID int64) bool {
	return IsTeamMember(t.OrgID, t.ID, userID)
}

func (t *Team) getRepositories(e Engine) (err error) {
	teamRepos := make([]*TeamRepo, 0, t.NumRepos)
	if err = x.
		Where("team_id=?", t.ID).
		Find(&teamRepos); err != nil {
		return fmt.Errorf("get team-repos: %v", err)
	}

	t.Repos = make([]*Repository, 0, len(teamRepos))
	for i := range teamRepos {
		repo, err := getRepositoryByID(e, teamRepos[i].RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryById(%d): %v", teamRepos[i].RepoID, err)
		}
		t.Repos = append(t.Repos, repo)
	}
	return nil
}

// GetRepositories returns all repositories in team of organization.
func (t *Team) GetRepositories() error {
	return t.getRepositories(x)
}

func (t *Team) getMembers(e Engine) (err error) {
	t.Members, err = getTeamMembers(e, t.ID)
	return err
}

// GetMembers returns all members in team of organization.
func (t *Team) GetMembers() (err error) {
	return t.getMembers(x)
}

// AddMember adds new membership of the team to the organization,
// the user will have membership to the organization automatically when needed.
func (t *Team) AddMember(userID int64) error {
	return AddTeamMember(t.OrgID, t.ID, userID)
}

// RemoveMember removes member from team of organization.
func (t *Team) RemoveMember(userID int64) error {
	return RemoveTeamMember(t.OrgID, t.ID, userID)
}

func (t *Team) hasRepository(e Engine, repoID int64) bool {
	return hasTeamRepo(e, t.OrgID, t.ID, repoID)
}

// HasRepository returns true if given repository belong to team.
func (t *Team) HasRepository(repoID int64) bool {
	return t.hasRepository(x, repoID)
}

func (t *Team) addRepository(e Engine, repo *Repository) (err error) {
	if err = addTeamRepo(e, t.OrgID, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos++
	if _, err = e.Id(t.ID).AllCols().Update(t); err != nil {
		return fmt.Errorf("update team: %v", err)
	}

	if err = repo.recalculateTeamAccesses(e, 0); err != nil {
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if err = t.getMembers(e); err != nil {
		return fmt.Errorf("getMembers: %v", err)
	}
	for _, u := range t.Members {
		if err = watchRepo(e, u.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}
	return nil
}

// AddRepository adds new repository to team of organization.
func (t *Team) AddRepository(repo *Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("Repository does not belong to organization")
	} else if t.HasRepository(repo.ID) {
		return nil
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.addRepository(sess, repo); err != nil {
		return err
	}

	return sess.Commit()
}

func (t *Team) removeRepository(e Engine, repo *Repository, recalculate bool) (err error) {
	if err = removeTeamRepo(e, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos--
	if _, err = e.Id(t.ID).AllCols().Update(t); err != nil {
		return err
	}

	// Don't need to recalculate when delete a repository from organization.
	if recalculate {
		if err = repo.recalculateTeamAccesses(e, t.ID); err != nil {
			return err
		}
	}

	if err = t.getMembers(e); err != nil {
		return fmt.Errorf("get team members: %v", err)
	}
	for _, u := range t.Members {
		has, err := hasAccess(e, u, repo, AccessModeRead)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = watchRepo(e, u.ID, repo.ID, false); err != nil {
			return err
		}
	}

	return nil
}

// RemoveRepository removes repository from team of organization.
func (t *Team) RemoveRepository(repoID int64) error {
	if !t.HasRepository(repoID) {
		return nil
	}

	repo, err := GetRepositoryByID(repoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.removeRepository(sess, repo, true); err != nil {
		return err
	}

	return sess.Commit()
}

// IsUsableTeamName tests if a name could be as team name
func IsUsableTeamName(name string) (err error) {
	var reservedTeamNames = []string{"new"}

	for i := range reservedTeamNames {
		if name == reservedTeamNames[i] {
			return ErrNameReserved{name}
		}
	}

	return nil
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

	has, err := x.Id(t.OrgID).Get(new(User))
	if err != nil {
		return err
	} else if !has {
		return ErrOrgNotExist
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err = x.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		Get(new(Team))
	if err != nil {
		return err
	} else if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(t); err != nil {
		sess.Rollback()
		return err
	}

	// Update organization number of teams.
	if _, err = sess.Exec("UPDATE `user` SET num_teams=num_teams+1 WHERE id = ?", t.OrgID); err != nil {
		sess.Rollback()
		return err
	}
	return sess.Commit()
}

func getTeam(e Engine, orgID int64, name string) (*Team, error) {
	t := &Team{
		OrgID:     orgID,
		LowerName: strings.ToLower(name),
	}
	has, err := e.Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist
	}
	return t, nil
}

// GetTeam returns team by given team name and organization.
func GetTeam(orgID int64, name string) (*Team, error) {
	return getTeam(x, orgID, name)
}

func getTeamByID(e Engine, teamID int64) (*Team, error) {
	t := new(Team)
	has, err := e.Id(teamID).Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist
	}
	return t, nil
}

// GetTeamByID returns team by given ID.
func GetTeamByID(teamID int64) (*Team, error) {
	return getTeamByID(x, teamID)
}

// UpdateTeam updates information of team.
func UpdateTeam(t *Team, authChanged bool) (err error) {
	if len(t.Name) == 0 {
		return errors.New("empty team name")
	}

	if len(t.Description) > 255 {
		t.Description = t.Description[:255]
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err := x.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		And("id!=?", t.ID).
		Get(new(Team))
	if err != nil {
		return err
	} else if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	if _, err = sess.Id(t.ID).AllCols().Update(t); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	// Update access for team members if needed.
	if authChanged {
		if err = t.getRepositories(sess); err != nil {
			return fmt.Errorf("getRepositories:%v", err)
		}

		for _, repo := range t.Repos {
			if err = repo.recalculateTeamAccesses(sess, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}
	}

	return sess.Commit()
}

// DeleteTeam deletes given team.
// It's caller's responsibility to assign organization ID.
func DeleteTeam(t *Team) error {
	if err := t.GetRepositories(); err != nil {
		return err
	}

	// Get organization.
	org, err := GetUserByID(t.OrgID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	// Delete all accesses.
	for _, repo := range t.Repos {
		if err = repo.recalculateTeamAccesses(sess, t.ID); err != nil {
			return err
		}
	}

	// Delete team-user.
	if _, err = sess.
		Where("org_id=?", org.ID).
		Where("team_id=?", t.ID).
		Delete(new(TeamUser)); err != nil {
		return err
	}

	// Delete team.
	if _, err = sess.Id(t.ID).Delete(new(Team)); err != nil {
		return err
	}
	// Update organization number of teams.
	if _, err = sess.Exec("UPDATE `user` SET num_teams=num_teams-1 WHERE id=?", t.OrgID); err != nil {
		return err
	}

	return sess.Commit()
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

func isTeamMember(e Engine, orgID, teamID, userID int64) bool {
	has, _ := e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("uid=?", userID).
		Get(new(TeamUser))
	return has
}

// IsTeamMember returns true if given user is a member of team.
func IsTeamMember(orgID, teamID, userID int64) bool {
	return isTeamMember(x, orgID, teamID, userID)
}

func getTeamMembers(e Engine, teamID int64) (_ []*User, err error) {
	teamUsers := make([]*TeamUser, 0, 10)
	if err = e.
		Where("team_id=?", teamID).
		Find(&teamUsers); err != nil {
		return nil, fmt.Errorf("get team-users: %v", err)
	}
	members := make([]*User, 0, len(teamUsers))
	for i := range teamUsers {
		member := new(User)
		if _, err = e.Id(teamUsers[i].UID).Get(member); err != nil {
			return nil, fmt.Errorf("get user '%d': %v", teamUsers[i].UID, err)
		}
		members = append(members, member)
	}
	return members, nil
}

// GetTeamMembers returns all members in given team of organization.
func GetTeamMembers(teamID int64) ([]*User, error) {
	return getTeamMembers(x, teamID)
}

func getUserTeams(e Engine, orgID, userID int64) ([]*Team, error) {
	tus := make([]*TeamUser, 0, 5)
	if err := e.
		Where("uid=?", userID).
		And("org_id=?", orgID).
		Find(&tus); err != nil {
		return nil, err
	}

	ts := make([]*Team, len(tus))
	for i, tu := range tus {
		t := new(Team)
		has, err := e.Id(tu.TeamID).Get(t)
		if err != nil {
			return nil, err
		} else if !has {
			return nil, ErrTeamNotExist
		}
		ts[i] = t
	}
	return ts, nil
}

// GetUserTeams returns all teams that user belongs to in given organization.
func GetUserTeams(orgID, userID int64) ([]*Team, error) {
	return getUserTeams(x, orgID, userID)
}

// AddTeamMember adds new membership of given team to given organization,
// the user will have membership to given organization automatically when needed.
func AddTeamMember(orgID, teamID, userID int64) error {
	if IsTeamMember(orgID, teamID, userID) {
		return nil
	}

	if err := AddOrgUser(orgID, userID); err != nil {
		return err
	}

	// Get team and its repositories.
	t, err := GetTeamByID(teamID)
	if err != nil {
		return err
	}
	t.NumMembers++

	if err = t.GetRepositories(); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	tu := &TeamUser{
		UID:    userID,
		OrgID:  orgID,
		TeamID: teamID,
	}
	if _, err = sess.Insert(tu); err != nil {
		return err
	} else if _, err = sess.Id(t.ID).Update(t); err != nil {
		return err
	}

	// Give access to team repositories.
	for _, repo := range t.Repos {
		if err = repo.recalculateTeamAccesses(sess, 0); err != nil {
			return err
		}
	}

	// We make sure it exists before.
	ou := new(OrgUser)
	if _, err = sess.
		Where("uid = ?", userID).
		And("org_id = ?", orgID).
		Get(ou); err != nil {
		return err
	}
	ou.NumTeams++
	if t.IsOwnerTeam() {
		ou.IsOwner = true
	}
	if _, err = sess.Id(ou.ID).AllCols().Update(ou); err != nil {
		return err
	}

	return sess.Commit()
}

func removeTeamMember(e Engine, orgID, teamID, userID int64) error {
	if !isTeamMember(e, orgID, teamID, userID) {
		return nil
	}

	// Get team and its repositories.
	t, err := getTeamByID(e, teamID)
	if err != nil {
		return err
	}

	// Check if the user to delete is the last member in owner team.
	if t.IsOwnerTeam() && t.NumMembers == 1 {
		return ErrLastOrgOwner{UID: userID}
	}

	t.NumMembers--

	if err = t.getRepositories(e); err != nil {
		return err
	}

	// Get organization.
	org, err := getUserByID(e, orgID)
	if err != nil {
		return err
	}

	tu := &TeamUser{
		UID:    userID,
		OrgID:  orgID,
		TeamID: teamID,
	}
	if _, err := e.Delete(tu); err != nil {
		return err
	} else if _, err = e.
		Id(t.ID).
		AllCols().
		Update(t); err != nil {
		return err
	}

	// Delete access to team repositories.
	for _, repo := range t.Repos {
		if err = repo.recalculateTeamAccesses(e, 0); err != nil {
			return err
		}
	}

	// This must exist.
	ou := new(OrgUser)
	_, err = e.
		Where("uid = ?", userID).
		And("org_id = ?", org.ID).
		Get(ou)
	if err != nil {
		return err
	}
	ou.NumTeams--
	if t.IsOwnerTeam() {
		ou.IsOwner = false
	}
	if _, err = e.
		Id(ou.ID).
		AllCols().
		Update(ou); err != nil {
		return err
	}
	return nil
}

// RemoveTeamMember removes member from given team of given organization.
func RemoveTeamMember(orgID, teamID, userID int64) error {
	sess := x.NewSession()
	defer sessionRelease(sess)
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := removeTeamMember(sess, orgID, teamID, userID); err != nil {
		return err
	}
	return sess.Commit()
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

func hasTeamRepo(e Engine, orgID, teamID, repoID int64) bool {
	has, _ := e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("repo_id=?", repoID).
		Get(new(TeamRepo))
	return has
}

// HasTeamRepo returns true if given repository belongs to team.
func HasTeamRepo(orgID, teamID, repoID int64) bool {
	return hasTeamRepo(x, orgID, teamID, repoID)
}

func addTeamRepo(e Engine, orgID, teamID, repoID int64) error {
	_, err := e.InsertOne(&TeamRepo{
		OrgID:  orgID,
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// AddTeamRepo adds new repository relation to team.
func AddTeamRepo(orgID, teamID, repoID int64) error {
	return addTeamRepo(x, orgID, teamID, repoID)
}

func removeTeamRepo(e Engine, teamID, repoID int64) error {
	_, err := e.Delete(&TeamRepo{
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// RemoveTeamRepo deletes repository relation to team.
func RemoveTeamRepo(teamID, repoID int64) error {
	return removeTeamRepo(x, teamID, repoID)
}
