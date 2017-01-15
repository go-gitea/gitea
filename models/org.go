// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Unknwon/com"
	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
)

var (
	// ErrOrgNotExist organization does not exist
	ErrOrgNotExist = errors.New("Organization does not exist")
	// ErrTeamNotExist team does not exist
	ErrTeamNotExist = errors.New("Team does not exist")
)

// IsOwnedBy returns true if given user is in the owner team.
func (org *User) IsOwnedBy(uid int64) bool {
	return IsOrganizationOwner(org.ID, uid)
}

// IsOrgMember returns true if given user is member of organization.
func (org *User) IsOrgMember(uid int64) bool {
	return org.IsOrganization() && IsOrganizationMember(org.ID, uid)
}

func (org *User) getTeam(e Engine, name string) (*Team, error) {
	return getTeam(e, org.ID, name)
}

// GetTeam returns named team of organization.
func (org *User) GetTeam(name string) (*Team, error) {
	return org.getTeam(x, name)
}

func (org *User) getOwnerTeam(e Engine) (*Team, error) {
	return org.getTeam(e, ownerTeamName)
}

// GetOwnerTeam returns owner team of organization.
func (org *User) GetOwnerTeam() (*Team, error) {
	return org.getOwnerTeam(x)
}

func (org *User) getTeams(e Engine) error {
	return e.
		Where("org_id=?", org.ID).
		OrderBy("CASE WHEN name LIKE '" + ownerTeamName + "' THEN '' ELSE name END").
		Find(&org.Teams)
}

// GetTeams returns all teams that belong to organization.
func (org *User) GetTeams() error {
	return org.getTeams(x)
}

// GetMembers returns all members of organization.
func (org *User) GetMembers() error {
	ous, err := GetOrgUsersByOrgID(org.ID)
	if err != nil {
		return err
	}

	var ids = make([]int64, len(ous))
	for i, ou := range ous {
		ids[i] = ou.UID
	}
	org.Members, err = GetUsersByIDs(ids)
	return err
}

// AddMember adds new member to organization.
func (org *User) AddMember(uid int64) error {
	return AddOrgUser(org.ID, uid)
}

// RemoveMember removes member from organization.
func (org *User) RemoveMember(uid int64) error {
	return RemoveOrgUser(org.ID, uid)
}

func (org *User) removeOrgRepo(e Engine, repoID int64) error {
	return removeOrgRepo(e, org.ID, repoID)
}

// RemoveOrgRepo removes all team-repository relations of organization.
func (org *User) RemoveOrgRepo(repoID int64) error {
	return org.removeOrgRepo(x, repoID)
}

// CreateOrganization creates record of a new organization.
func CreateOrganization(org, owner *User) (err error) {
	if !owner.CanCreateOrganization() {
		return ErrUserNotAllowedCreateOrg{}
	}

	if err = IsUsableUsername(org.Name); err != nil {
		return err
	}

	isExist, err := IsUserExist(0, org.Name)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{org.Name}
	}

	org.LowerName = strings.ToLower(org.Name)
	if org.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	if org.Salt, err = GetUserSalt(); err != nil {
		return err
	}
	org.UseCustomAvatar = true
	org.MaxRepoCreation = -1
	org.NumTeams = 1
	org.NumMembers = 1

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(org); err != nil {
		return fmt.Errorf("insert organization: %v", err)
	}
	org.GenerateRandomAvatar()

	// Add initial creator to organization and owner team.
	if _, err = sess.Insert(&OrgUser{
		UID:      owner.ID,
		OrgID:    org.ID,
		IsOwner:  true,
		NumTeams: 1,
	}); err != nil {
		return fmt.Errorf("insert org-user relation: %v", err)
	}

	// Create default owner team.
	t := &Team{
		OrgID:      org.ID,
		LowerName:  strings.ToLower(ownerTeamName),
		Name:       ownerTeamName,
		Authorize:  AccessModeOwner,
		NumMembers: 1,
	}
	if _, err = sess.Insert(t); err != nil {
		return fmt.Errorf("insert owner team: %v", err)
	}

	if _, err = sess.Insert(&TeamUser{
		UID:    owner.ID,
		OrgID:  org.ID,
		TeamID: t.ID,
	}); err != nil {
		return fmt.Errorf("insert team-user relation: %v", err)
	}

	if err = os.MkdirAll(UserPath(org.Name), os.ModePerm); err != nil {
		return fmt.Errorf("create directory: %v", err)
	}

	return sess.Commit()
}

// GetOrgByName returns organization by given name.
func GetOrgByName(name string) (*User, error) {
	if len(name) == 0 {
		return nil, ErrOrgNotExist
	}
	u := &User{
		LowerName: strings.ToLower(name),
		Type:      UserTypeOrganization,
	}
	has, err := x.Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgNotExist
	}
	return u, nil
}

// CountOrganizations returns number of organizations.
func CountOrganizations() int64 {
	count, _ := x.
		Where("type=1").
		Count(new(User))
	return count
}

// Organizations returns number of organizations in given page.
func Organizations(opts *SearchUserOptions) ([]*User, error) {
	orgs := make([]*User, 0, opts.PageSize)

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "name ASC"
	}

	sess := x.
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Where("type=1")

	return orgs, sess.
		OrderBy(opts.OrderBy).
		Find(&orgs)
}

// DeleteOrganization completely and permanently deletes everything of organization.
func DeleteOrganization(org *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()

	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteOrg(sess, org); err != nil {
		if IsErrUserOwnRepos(err) {
			return err
		} else if err != nil {
			return fmt.Errorf("deleteOrg: %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	return nil
}

func deleteOrg(e *xorm.Session, u *User) error {
	if !u.IsOrganization() {
		return fmt.Errorf("You can't delete none organization user: %s", u.Name)
	}

	// Check ownership of repository.
	count, err := getRepositoryCount(e, u)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return ErrUserOwnRepos{UID: u.ID}
	}

	if err := deleteBeans(e,
		&Team{OrgID: u.ID},
		&OrgUser{OrgID: u.ID},
		&TeamUser{OrgID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	if _, err = e.Id(u.ID).Delete(new(User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	// FIXME: system notice
	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := UserPath(u.Name)

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("Fail to RemoveAll %s: %v", path, err)
	}

	avatarPath := u.CustomAvatarPath()
	if com.IsExist(avatarPath) {
		if err := os.Remove(avatarPath); err != nil {
			return fmt.Errorf("Fail to remove %s: %v", avatarPath, err)
		}
	}

	return nil
}

// ________                ____ ___
// \_____  \_______  ____ |    |   \______ ___________
//  /   |   \_  __ \/ ___\|    |   /  ___// __ \_  __ \
// /    |    \  | \/ /_/  >    |  /\___ \\  ___/|  | \/
// \_______  /__|  \___  /|______//____  >\___  >__|
//         \/     /_____/              \/     \/

// OrgUser represents an organization-user relation.
type OrgUser struct {
	ID       int64 `xorm:"pk autoincr"`
	UID      int64 `xorm:"INDEX UNIQUE(s)"`
	OrgID    int64 `xorm:"INDEX UNIQUE(s)"`
	IsPublic bool  `xorm:"INDEX"`
	IsOwner  bool
	NumTeams int
}

// IsOrganizationOwner returns true if given user is in the owner team.
func IsOrganizationOwner(orgID, uid int64) bool {
	has, _ := x.
		Where("is_owner=?", true).
		And("uid=?", uid).
		And("org_id=?", orgID).
		Get(new(OrgUser))
	return has
}

// IsOrganizationMember returns true if given user is member of organization.
func IsOrganizationMember(orgID, uid int64) bool {
	has, _ := x.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Get(new(OrgUser))
	return has
}

// IsPublicMembership returns true if given user public his/her membership.
func IsPublicMembership(orgID, uid int64) bool {
	has, _ := x.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		And("is_public=?", true).
		Get(new(OrgUser))
	return has
}

func getOrgsByUserID(sess *xorm.Session, userID int64, showAll bool) ([]*User, error) {
	orgs := make([]*User, 0, 10)
	if !showAll {
		sess.And("`org_user`.is_public=?", true)
	}
	return orgs, sess.
		And("`org_user`.uid=?", userID).
		Join("INNER", "`org_user`", "`org_user`.org_id=`user`.id").
		Asc("`user`.name").
		Find(&orgs)
}

// GetOrgsByUserID returns a list of organizations that the given user ID
// has joined.
func GetOrgsByUserID(userID int64, showAll bool) ([]*User, error) {
	return getOrgsByUserID(x.NewSession(), userID, showAll)
}

// GetOrgsByUserIDDesc returns a list of organizations that the given user ID
// has joined, ordered descending by the given condition.
func GetOrgsByUserIDDesc(userID int64, desc string, showAll bool) ([]*User, error) {
	return getOrgsByUserID(x.NewSession().Desc(desc), userID, showAll)
}

func getOwnedOrgsByUserID(sess *xorm.Session, userID int64) ([]*User, error) {
	orgs := make([]*User, 0, 10)
	return orgs, sess.
		Where("`org_user`.uid=?", userID).
		And("`org_user`.is_owner=?", true).
		Join("INNER", "`org_user`", "`org_user`.org_id=`user`.id").
		Asc("`user`.name").
		Find(&orgs)
}

// GetOwnedOrgsByUserID returns a list of organizations are owned by given user ID.
func GetOwnedOrgsByUserID(userID int64) ([]*User, error) {
	sess := x.NewSession()
	return getOwnedOrgsByUserID(sess, userID)
}

// GetOwnedOrgsByUserIDDesc returns a list of organizations are owned by
// given user ID, ordered descending by the given condition.
func GetOwnedOrgsByUserIDDesc(userID int64, desc string) ([]*User, error) {
	sess := x.NewSession()
	return getOwnedOrgsByUserID(sess.Desc(desc), userID)
}

// GetOrgUsersByUserID returns all organization-user relations by user ID.
func GetOrgUsersByUserID(uid int64, all bool) ([]*OrgUser, error) {
	ous := make([]*OrgUser, 0, 10)
	sess := x.
		Join("LEFT", "user", "`org_user`.org_id=`user`.id").
		Where("`org_user`.uid=?", uid)
	if !all {
		// Only show public organizations
		sess.And("is_public=?", true)
	}
	err := sess.
		Asc("`user`.name").
		Find(&ous)
	return ous, err
}

// GetOrgUsersByOrgID returns all organization-user relations by organization ID.
func GetOrgUsersByOrgID(orgID int64) ([]*OrgUser, error) {
	ous := make([]*OrgUser, 0, 10)
	err := x.
		Where("org_id=?", orgID).
		Find(&ous)
	return ous, err
}

// ChangeOrgUserStatus changes public or private membership status.
func ChangeOrgUserStatus(orgID, uid int64, public bool) error {
	ou := new(OrgUser)
	has, err := x.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return err
	} else if !has {
		return nil
	}

	ou.IsPublic = public
	_, err = x.Id(ou.ID).AllCols().Update(ou)
	return err
}

// AddOrgUser adds new user to given organization.
func AddOrgUser(orgID, uid int64) error {
	if IsOrganizationMember(orgID, uid) {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	ou := &OrgUser{
		UID:   uid,
		OrgID: orgID,
	}

	if _, err := sess.Insert(ou); err != nil {
		sess.Rollback()
		return err
	} else if _, err = sess.Exec("UPDATE `user` SET num_members = num_members + 1 WHERE id = ?", orgID); err != nil {
		sess.Rollback()
		return err
	}

	return sess.Commit()
}

// RemoveOrgUser removes user from given organization.
func RemoveOrgUser(orgID, userID int64) error {
	ou := new(OrgUser)

	has, err := x.
		Where("uid=?", userID).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return fmt.Errorf("get org-user: %v", err)
	} else if !has {
		return nil
	}

	user, err := GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", userID, err)
	}
	org, err := GetUserByID(orgID)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", orgID, err)
	}

	// FIXME: only need to get IDs here, not all fields of repository.
	repos, _, err := org.GetUserRepositories(user.ID, 1, org.NumRepos)
	if err != nil {
		return fmt.Errorf("GetUserRepositories [%d]: %v", user.ID, err)
	}

	// Check if the user to delete is the last member in owner team.
	if IsOrganizationOwner(orgID, userID) {
		t, err := org.GetOwnerTeam()
		if err != nil {
			return err
		}
		if t.NumMembers == 1 {
			return ErrLastOrgOwner{UID: userID}
		}
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Id(ou.ID).Delete(ou); err != nil {
		return err
	} else if _, err = sess.Exec("UPDATE `user` SET num_members=num_members-1 WHERE id=?", orgID); err != nil {
		return err
	}

	// Delete all repository accesses and unwatch them.
	repoIDs := make([]int64, len(repos))
	for i := range repos {
		repoIDs = append(repoIDs, repos[i].ID)
		if err = watchRepo(sess, user.ID, repos[i].ID, false); err != nil {
			return err
		}
	}

	if len(repoIDs) > 0 {
		if _, err = sess.
			Where("user_id = ?", user.ID).
			In("repo_id", repoIDs).
			Delete(new(Access)); err != nil {
			return err
		}
	}

	// Delete member in his/her teams.
	teams, err := getUserTeams(sess, org.ID, user.ID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err = removeTeamMember(sess, org.ID, t.ID, user.ID); err != nil {
			return err
		}
	}

	return sess.Commit()
}

func removeOrgRepo(e Engine, orgID, repoID int64) error {
	_, err := e.Delete(&TeamRepo{
		OrgID:  orgID,
		RepoID: repoID,
	})
	return err
}

// RemoveOrgRepo removes all team-repository relations of given organization.
func RemoveOrgRepo(orgID, repoID int64) error {
	return removeOrgRepo(x, orgID, repoID)
}

func (org *User) getUserTeams(e Engine, userID int64, cols ...string) ([]*Team, error) {
	teams := make([]*Team, 0, org.NumTeams)
	return teams, e.
		Where("`team_user`.org_id = ?", org.ID).
		Join("INNER", "team_user", "`team_user`.team_id = team.id").
		Join("INNER", "user", "`user`.id=team_user.uid").
		And("`team_user`.uid = ?", userID).
		Asc("`user`.name").
		Cols(cols...).
		Find(&teams)
}

// GetUserTeamIDs returns of all team IDs of the organization that user is member of.
func (org *User) GetUserTeamIDs(userID int64) ([]int64, error) {
	teams, err := org.getUserTeams(x, userID, "team.id")
	if err != nil {
		return nil, fmt.Errorf("getUserTeams [%d]: %v", userID, err)
	}

	teamIDs := make([]int64, len(teams))
	for i := range teams {
		teamIDs[i] = teams[i].ID
	}
	return teamIDs, nil
}

// GetUserTeams returns all teams that belong to user,
// and that the user has joined.
func (org *User) GetUserTeams(userID int64) ([]*Team, error) {
	return org.getUserTeams(x, userID)
}

// GetUserRepositories returns a range of repositories in organization
// that the user with the given userID has access to,
// and total number of records based on given condition.
func (org *User) GetUserRepositories(userID int64, page, pageSize int) ([]*Repository, int64, error) {
	var cond builder.Cond = builder.Eq{
		"`repository`.owner_id":   org.ID,
		"`repository`.is_private": false,
	}

	teamIDs, err := org.GetUserTeamIDs(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("GetUserTeamIDs: %v", err)
	}

	if len(teamIDs) > 0 {
		cond = cond.Or(builder.In("team_repo.team_id", teamIDs))
	}

	if page <= 0 {
		page = 1
	}

	repos := make([]*Repository, 0, pageSize)

	if err := x.
		Select("`repository`.id").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(cond).
		GroupBy("`repository`.id,`repository`.updated_unix").
		OrderBy("updated_unix DESC").
		Limit(pageSize, (page-1)*pageSize).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("get repository ids: %v", err)
	}

	repoIDs := make([]int64,pageSize)
	for i := range repos {
		repoIDs[i] = repos[i].ID
	}

	if err := x.
		Select("`repository`.*").
		Where(builder.In("`repository`.id",repoIDs)).
		Find(&repos); err!=nil {
		return nil, 0, fmt.Errorf("get repositories: %v", err)
	}

	repoCount, err := x.
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(cond).
		GroupBy("`repository`.id").
		Count(&Repository{})
	if err != nil {
		return nil, 0, fmt.Errorf("count user repositories in organization: %v", err)
	}

	return repos, repoCount, nil
}

// GetUserMirrorRepositories returns mirror repositories of the user
// that the user with the given userID has access to.
func (org *User) GetUserMirrorRepositories(userID int64) ([]*Repository, error) {
	teamIDs, err := org.GetUserTeamIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("GetUserTeamIDs: %v", err)
	}
	if len(teamIDs) == 0 {
		teamIDs = []int64{-1}
	}

	repos := make([]*Repository, 0, 10)
	return repos, x.
		Select("`repository`.*").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id AND `repository`.is_mirror=?", true).
		Where("(`repository`.owner_id=? AND `repository`.is_private=?)", org.ID, false).
		Or(builder.In("team_repo.team_id", teamIDs)).
		GroupBy("`repository`.id").
		OrderBy("updated_unix DESC").
		Find(&repos)
}
