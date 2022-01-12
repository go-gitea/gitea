// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// Permission contains all the permissions related variables to a repository for a user
type Permission struct {
	AccessMode perm_model.AccessMode
	Units      []*repo_model.RepoUnit
	UnitsMode  map[unit.Type]perm_model.AccessMode
}

// IsOwner returns true if current user is the owner of repository.
func (p *Permission) IsOwner() bool {
	return p.AccessMode >= perm_model.AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (p *Permission) IsAdmin() bool {
	return p.AccessMode >= perm_model.AccessModeAdmin
}

// HasAccess returns true if the current user has at least read access to any unit of this repository
func (p *Permission) HasAccess() bool {
	if p.UnitsMode == nil {
		return p.AccessMode >= perm_model.AccessModeRead
	}
	return len(p.UnitsMode) > 0
}

// UnitAccessMode returns current user accessmode to the specify unit of the repository
func (p *Permission) UnitAccessMode(unitType unit.Type) perm_model.AccessMode {
	if p.UnitsMode == nil {
		for _, u := range p.Units {
			if u.Type == unitType {
				return p.AccessMode
			}
		}
		return perm_model.AccessModeNone
	}
	return p.UnitsMode[unitType]
}

// CanAccess returns true if user has mode access to the unit of the repository
func (p *Permission) CanAccess(mode perm_model.AccessMode, unitType unit.Type) bool {
	return p.UnitAccessMode(unitType) >= mode
}

// CanAccessAny returns true if user has mode access to any of the units of the repository
func (p *Permission) CanAccessAny(mode perm_model.AccessMode, unitTypes ...unit.Type) bool {
	for _, u := range unitTypes {
		if p.CanAccess(mode, u) {
			return true
		}
	}
	return false
}

// CanRead returns true if user could read to this unit
func (p *Permission) CanRead(unitType unit.Type) bool {
	return p.CanAccess(perm_model.AccessModeRead, unitType)
}

// CanReadAny returns true if user has read access to any of the units of the repository
func (p *Permission) CanReadAny(unitTypes ...unit.Type) bool {
	return p.CanAccessAny(perm_model.AccessModeRead, unitTypes...)
}

// CanReadIssuesOrPulls returns true if isPull is true and user could read pull requests and
// returns true if isPull is false and user could read to issues
func (p *Permission) CanReadIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanRead(unit.TypePullRequests)
	}
	return p.CanRead(unit.TypeIssues)
}

// CanWrite returns true if user could write to this unit
func (p *Permission) CanWrite(unitType unit.Type) bool {
	return p.CanAccess(perm_model.AccessModeWrite, unitType)
}

// CanWriteIssuesOrPulls returns true if isPull is true and user could write to pull requests and
// returns true if isPull is false and user could write to issues
func (p *Permission) CanWriteIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanWrite(unit.TypePullRequests)
	}
	return p.CanWrite(unit.TypeIssues)
}

// ColorFormat writes a colored string for these Permissions
func (p *Permission) ColorFormat(s fmt.State) {
	noColor := log.ColorBytes(log.Reset)

	format := "perm_model.AccessMode: %-v, %d Units, %d UnitsMode(s): [ "
	args := []interface{}{
		p.AccessMode,
		log.NewColoredValueBytes(len(p.Units), &noColor),
		log.NewColoredValueBytes(len(p.UnitsMode), &noColor),
	}
	if s.Flag('+') {
		for i, unit := range p.Units {
			config := ""
			if unit.Config != nil {
				configBytes, err := unit.Config.ToDB()
				config = string(configBytes)
				if err != nil {
					config = err.Error()
				}
			}
			format += "\nUnits[%d]: ID: %d RepoID: %d Type: %-v Config: %s"
			args = append(args,
				log.NewColoredValueBytes(i, &noColor),
				log.NewColoredIDValue(unit.ID),
				log.NewColoredIDValue(unit.RepoID),
				unit.Type,
				config)
		}
		for key, value := range p.UnitsMode {
			format += "\nUnitMode[%-v]: %-v"
			args = append(args,
				key,
				value)
		}
	} else {
		format += "..."
	}
	format += " ]"
	log.ColorFprintf(s, format, args...)
}

// GetUserRepoPermission returns the user permissions to the repository
func GetUserRepoPermission(repo *repo_model.Repository, user *user_model.User) (Permission, error) {
	return getUserRepoPermission(db.DefaultContext, repo, user)
}

func getUserRepoPermission(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (perm Permission, err error) {
	if log.IsTrace() {
		defer func() {
			if user == nil {
				log.Trace("Permission Loaded for anonymous user in %-v:\nPermissions: %-+v",
					repo,
					perm)
				return
			}
			log.Trace("Permission Loaded for %-v in %-v:\nPermissions: %-+v",
				user,
				repo,
				perm)
		}()
	}
	// anonymous user visit private repo.
	// TODO: anonymous user visit public unit of private repo???
	if user == nil && repo.IsPrivate {
		perm.AccessMode = perm_model.AccessModeNone
		return
	}

	e := db.GetEngine(ctx)

	var is bool
	if user != nil {
		is, err = isCollaborator(e, repo.ID, user.ID)
		if err != nil {
			return perm, err
		}
	}

	if err = repo.GetOwner(ctx); err != nil {
		return
	}

	// Prevent strangers from checking out public repo of private organization/users
	// Allow user if they are collaborator of a repo within a private user or a private organization but not a member of the organization itself
	if !hasOrgOrUserVisible(e, repo.Owner, user) && !is {
		perm.AccessMode = perm_model.AccessModeNone
		return
	}

	if err = repo.LoadUnits(ctx); err != nil {
		return
	}

	perm.Units = repo.Units

	// anonymous visit public repo
	if user == nil {
		perm.AccessMode = perm_model.AccessModeRead
		return
	}

	// Admin or the owner has super access to the repository
	if user.IsAdmin || user.ID == repo.OwnerID {
		perm.AccessMode = perm_model.AccessModeOwner
		return
	}

	// plain user
	perm.AccessMode, err = accessLevel(e, user, repo)
	if err != nil {
		return
	}

	if err = repo.GetOwner(ctx); err != nil {
		return
	}
	if !repo.Owner.IsOrganization() {
		return
	}

	perm.UnitsMode = make(map[unit.Type]perm_model.AccessMode)

	// Collaborators on organization
	if is {
		for _, u := range repo.Units {
			perm.UnitsMode[u.Type] = perm.AccessMode
		}
	}

	// get units mode from teams
	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return
	}

	// if user in an owner team
	for _, team := range teams {
		if team.AccessMode >= perm_model.AccessModeAdmin {
			perm.AccessMode = perm_model.AccessModeOwner
			perm.UnitsMode = nil
			return
		}
	}

	for _, u := range repo.Units {
		var found bool
		for _, team := range teams {
			teamMode := team.unitAccessMode(e, u.Type)
			if teamMode > perm_model.AccessModeNone {
				m := perm.UnitsMode[u.Type]
				if m < teamMode {
					perm.UnitsMode[u.Type] = teamMode
				}
				found = true
			}
		}

		// for a public repo on an organization, a non-restricted user has read permission on non-team defined units.
		if !found && !repo.IsPrivate && !user.IsRestricted {
			if _, ok := perm.UnitsMode[u.Type]; !ok {
				perm.UnitsMode[u.Type] = perm_model.AccessModeRead
			}
		}
	}

	// remove no permission units
	perm.Units = make([]*repo_model.RepoUnit, 0, len(repo.Units))
	for t := range perm.UnitsMode {
		for _, u := range repo.Units {
			if u.Type == t {
				perm.Units = append(perm.Units, u)
			}
		}
	}

	return
}

// IsUserRealRepoAdmin check if this user is real repo admin
func IsUserRealRepoAdmin(repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if repo.OwnerID == user.ID {
		return true, nil
	}

	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return false, err
	}

	accessMode, err := accessLevel(db.GetEngine(db.DefaultContext), user, repo)
	if err != nil {
		return false, err
	}

	return accessMode >= perm_model.AccessModeAdmin, nil
}

// IsUserRepoAdmin return true if user has admin right of a repo
func IsUserRepoAdmin(repo *repo_model.Repository, user *user_model.User) (bool, error) {
	return isUserRepoAdmin(db.GetEngine(db.DefaultContext), repo, user)
}

func isUserRepoAdmin(e db.Engine, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if user == nil || repo == nil {
		return false, nil
	}
	if user.IsAdmin {
		return true, nil
	}

	mode, err := accessLevel(e, user, repo)
	if err != nil {
		return false, err
	}
	if mode >= perm_model.AccessModeAdmin {
		return true, nil
	}

	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return false, err
	}

	for _, team := range teams {
		if team.AccessMode >= perm_model.AccessModeAdmin {
			return true, nil
		}
	}
	return false, nil
}

// AccessLevel returns the Access a user has to a repository. Will return NoneAccess if the
// user does not have access.
func AccessLevel(user *user_model.User, repo *repo_model.Repository) (perm_model.AccessMode, error) {
	return accessLevelUnit(db.DefaultContext, user, repo, unit.TypeCode)
}

// AccessLevelUnit returns the Access a user has to a repository's. Will return NoneAccess if the
// user does not have access.
func AccessLevelUnit(user *user_model.User, repo *repo_model.Repository, unitType unit.Type) (perm_model.AccessMode, error) {
	return accessLevelUnit(db.DefaultContext, user, repo, unitType)
}

func accessLevelUnit(ctx context.Context, user *user_model.User, repo *repo_model.Repository, unitType unit.Type) (perm_model.AccessMode, error) {
	perm, err := getUserRepoPermission(ctx, repo, user)
	if err != nil {
		return perm_model.AccessModeNone, err
	}
	return perm.UnitAccessMode(unitType), nil
}

func hasAccessUnit(ctx context.Context, user *user_model.User, repo *repo_model.Repository, unitType unit.Type, testMode perm_model.AccessMode) (bool, error) {
	mode, err := accessLevelUnit(ctx, user, repo, unitType)
	return testMode <= mode, err
}

// HasAccessUnit returns true if user has testMode to the unit of the repository
func HasAccessUnit(user *user_model.User, repo *repo_model.Repository, unitType unit.Type, testMode perm_model.AccessMode) (bool, error) {
	return hasAccessUnit(db.DefaultContext, user, repo, unitType, testMode)
}

// CanBeAssigned return true if user can be assigned to issue or pull requests in repo
// Currently any write access (code, issues or pr's) is assignable, to match assignee list in user interface.
// FIXME: user could send PullRequest also could be assigned???
func CanBeAssigned(user *user_model.User, repo *repo_model.Repository, isPull bool) (bool, error) {
	return canBeAssigned(db.DefaultContext, user, repo, isPull)
}

func canBeAssigned(ctx context.Context, user *user_model.User, repo *repo_model.Repository, _ bool) (bool, error) {
	if user.IsOrganization() {
		return false, fmt.Errorf("Organization can't be added as assignee [user_id: %d, repo_id: %d]", user.ID, repo.ID)
	}
	perm, err := getUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return perm.CanAccessAny(perm_model.AccessModeWrite, unit.TypeCode, unit.TypeIssues, unit.TypePullRequests), nil
}

func hasAccess(ctx context.Context, userID int64, repo *repo_model.Repository) (bool, error) {
	var user *user_model.User
	var err error
	if userID > 0 {
		user, err = user_model.GetUserByIDEngine(db.GetEngine(ctx), userID)
		if err != nil {
			return false, err
		}
	}
	perm, err := getUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return perm.HasAccess(), nil
}

// HasAccess returns true if user has access to repo
func HasAccess(userID int64, repo *repo_model.Repository) (bool, error) {
	return hasAccess(db.DefaultContext, userID, repo)
}

// GetRepoReaders returns all users that have explicit read access or higher to the repository.
func GetRepoReaders(repo *repo_model.Repository) (_ []*user_model.User, err error) {
	return getUsersWithAccessMode(db.DefaultContext, repo, perm_model.AccessModeRead)
}

// GetRepoWriters returns all users that have write access to the repository.
func GetRepoWriters(repo *repo_model.Repository) (_ []*user_model.User, err error) {
	return getUsersWithAccessMode(db.DefaultContext, repo, perm_model.AccessModeWrite)
}

// IsRepoReader returns true if user has explicit read access or higher to the repository.
func IsRepoReader(repo *repo_model.Repository, userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	return db.GetEngine(db.DefaultContext).Where("repo_id = ? AND user_id = ? AND mode >= ?", repo.ID, userID, perm_model.AccessModeRead).Get(&Access{})
}
