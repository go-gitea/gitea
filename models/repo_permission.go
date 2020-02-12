// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
)

// Permission contains all the permissions related variables to a repository for a user
type Permission struct {
	MinAccessMode AccessMode // Name change to ensure that this name is only used in repo_permission.go
	Units         []*RepoUnit
	UnitsMode     map[UnitType]AccessMode
	isOwner       bool
	isAdmin       bool
}

// IsOwner returns true if current user is the owner of repository.
func (p *Permission) IsOwner() bool {
	return p.MinAccessMode >= AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (p *Permission) IsAdmin() bool {
	return p.MinAccessMode >= AccessModeAdmin
}

// HasAccess returns true if the current user has at least read access to any unit of this repository
func (p *Permission) HasAccess() bool {
	if p.UnitsMode == nil {
		return p.MinAccessMode >= AccessModeRead
	}
	return len(p.UnitsMode) > 0
}

// UnitAccessMode returns current user accessmode to the specify unit of the repository
func (p *Permission) UnitAccessMode(unitType UnitType) AccessMode {
	if p.UnitsMode == nil {
		for _, u := range p.Units {
			if u.Type == unitType {
				return p.MinAccessMode
			}
		}
		return AccessModeNone
	}
	return p.UnitsMode[unitType]
}

// CanAccess returns true if user has mode access to the unit of the repository
func (p *Permission) CanAccess(mode AccessMode, unitType UnitType) bool {
	return p.UnitAccessMode(unitType) >= mode
}

// CanAccessAny returns true if user has mode access to any of the units of the repository
func (p *Permission) CanAccessAny(mode AccessMode, unitTypes ...UnitType) bool {
	for _, u := range unitTypes {
		if p.CanAccess(mode, u) {
			return true
		}
	}
	return false
}

// CanRead returns true if user could read to this unit
func (p *Permission) CanRead(unitType UnitType) bool {
	return p.CanAccess(AccessModeRead, unitType)
}

// CanReadAny returns true if user has read access to any of the units of the repository
func (p *Permission) CanReadAny(unitTypes ...UnitType) bool {
	return p.CanAccessAny(AccessModeRead, unitTypes...)
}

// CanReadIssuesOrPulls returns true if isPull is true and user could read pull requests and
// returns true if isPull is false and user could read to issues
func (p *Permission) CanReadIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanRead(UnitTypePullRequests)
	}
	return p.CanRead(UnitTypeIssues)
}

// CanWrite returns true if user could write to this unit
func (p *Permission) CanWrite(unitType UnitType) bool {
	return p.CanAccess(AccessModeWrite, unitType)
}

// CanWriteIssuesOrPulls returns true if isPull is true and user could write to pull requests and
// returns true if isPull is false and user could write to issues
func (p *Permission) CanWriteIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanWrite(UnitTypePullRequests)
	}
	return p.CanWrite(UnitTypeIssues)
}

// ColorFormat writes a colored string for these Permissions
func (p *Permission) ColorFormat(s fmt.State) {
	noColor := log.ColorBytes(log.Reset)

	format := "AccessMode: %-v, %d Units, %d UnitsMode(s): [ "
	args := []interface{}{
		p.MinAccessMode,
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
func GetUserRepoPermission(repo *Repository, user *User) (Permission, error) {
	return getUserRepoPermission(x, repo, user)
}

func getUserRepoPermissionNew(e Engine, repo *Repository, user *User) (perm Permission, err error) {
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

	type unitModePair struct {
		Type UnitType
		Mode AccessMode
	}
	pairs := make([]*unitModePair, 0, 10)
	sess := e.Table(&UserRepoUnit{}).Where("repo_id = ?", repo.ID)
	if user == nil || user.ID == 0 {
		sess = sess.And("user_id = ?", UserRepoUnitAnyUser)
	} else if user.IsRestricted {
		sess = sess.And("user_id = ?", user.ID)
	} else if user.IsAdmin {
		sess = sess.And(sess.In("user_id", UserRepoUnitAdminUser, user.ID))
	} else {
		sess = sess.And(sess.In("user_id", UserRepoUnitLoggedInUser, user.ID))
	}
	if err = sess.Select("user_repo_unit.`type` AS `unit`, MAX(user_repo_unit.`mode`) AS `mode").
		GroupBy("user_repo_unit.`type`").
		Find(pairs); err != nil {
		return
	}

	if len(pairs) == 0 {
		// No permissions
		return
	}

	// Only process units a user can actually be permitted or denied (i.e. are "checkable")
	// There are units that are only meant for repository level control, so they are not
	// checked here.

	// The Permission struct will contain:
	// isAdmin			true if all **checkable** units are >= AccessModeAdmin
	// isOwner			true if all **checkable** units are >= AccessModeOwner
	// Units			a list of all **checkable** units in the repository
	// UnitsMode		what access mode was granted to the user for each unit
	// MinAccessMode	value of the minimum access granted to the user for this repository (all units considered)

	// Note that MinAccessMode will be AccessModeNone if the user has been denied
	// access to any one unit of the repository.

	// FIXME: all IsAdmin() and IsOwner() calls should specify what unit is intended

	perm.Units = make([]*RepoUnit, 0, len(repo.Units))
	perm.UnitsMode = make(map[UnitType]AccessMode, len(repo.Units))
	perm.isOwner = true // As long as no unit contradicts this
	perm.isAdmin = true // As long as no unit contradicts this
	firstUnit := true

	// FIXME: GAP: What are the units that are effectively required here?
	for _, st := range UserRepoUnitSelectableTypes {
		for _, unit := range perm.Units {
			if unit.Type == st {
				perm.Units = append(perm.Units, unit)
				mode := AccessModeNone
				for _, ump := range pairs {
					if ump.Type == st {
						mode = ump.Mode
						break
					}
				}
				perm.UnitsMode[unit.Type] = mode
				if firstUnit || mode < perm.MinAccessMode {
					perm.MinAccessMode = mode
				}
				firstUnit = false
				if mode < AccessModeOwner {
					perm.isOwner = false
				}
				if mode < AccessModeAdmin {
					perm.isAdmin = false
				}
				break
			}
		}
	}

	// If we couldn't compute any units, the user is not owner or admin,
	// (i.e. through team permissions, etc.) unless they are actually
	// the owner or a site admin.
	// We check these values on top of user_repo_unit because the admin/owner
	// statuses are used to check actions beyond those in UserRepoUnitSelectableTypes.
	if user.IsAdmin {
		perm.isAdmin = true
	} else if len(perm.Units) == 0 {
		perm.isAdmin = false
	}

	if user.ID == repo.OwnerID {
		perm.isOwner = true
	} else if len(perm.Units) == 0 {
		perm.isOwner = false
	}

	return
}

func getUserRepoPermission(e Engine, repo *Repository, user *User) (perm Permission, err error) {
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
		perm.MinAccessMode = AccessModeNone
		return
	}

	var isCollaborator bool
	if user != nil {
		isCollaborator, err = repo.isCollaborator(e, user.ID)
		if err != nil {
			return perm, err
		}
	}

	if err = repo.getOwner(e); err != nil {
		return
	}

	// Prevent strangers from checking out public repo of private orginization
	// Allow user if they are collaborator of a repo within a private orginization but not a member of the orginization itself
	if repo.Owner.IsOrganization() && !HasOrgVisible(repo.Owner, user) && !isCollaborator {
		perm.MinAccessMode = AccessModeNone
		return
	}

	if err = repo.getUnits(e); err != nil {
		return
	}

	perm.Units = repo.Units

	// anonymous visit public repo
	if user == nil {
		perm.MinAccessMode = AccessModeRead
		return
	}

	// Admin or the owner has super access to the repository
	if user.IsAdmin || user.ID == repo.OwnerID {
		perm.MinAccessMode = AccessModeOwner
		return
	}

	// plain user
	perm.MinAccessMode, err = accessLevel(e, user, repo)
	if err != nil {
		return
	}

	if err = repo.getOwner(e); err != nil {
		return
	}
	if !repo.Owner.IsOrganization() {
		return
	}

	perm.UnitsMode = make(map[UnitType]AccessMode)

	// Collaborators on organization
	if isCollaborator {
		for _, u := range repo.Units {
			perm.UnitsMode[u.Type] = perm.MinAccessMode
		}
	}

	// get units mode from teams
	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return
	}

	// if user in an owner team
	for _, team := range teams {
		if team.Authorize >= AccessModeOwner {
			perm.MinAccessMode = AccessModeOwner
			perm.UnitsMode = nil
			return
		}
	}

	for _, u := range repo.Units {
		var found bool
		for _, team := range teams {
			if team.unitEnabled(e, u.Type) {
				m := perm.UnitsMode[u.Type]
				if m < team.Authorize {
					perm.UnitsMode[u.Type] = team.Authorize
				}
				found = true
			}
		}

		// for a public repo on an organization, a non-restricted user has read permission on non-team defined units.
		if !found && !repo.IsPrivate && !user.IsRestricted {
			if _, ok := perm.UnitsMode[u.Type]; !ok {
				perm.UnitsMode[u.Type] = AccessModeRead
			}
		}
	}

	// remove no permission units
	perm.Units = make([]*RepoUnit, 0, len(repo.Units))
	for t := range perm.UnitsMode {
		for _, u := range repo.Units {
			if u.Type == t {
				perm.Units = append(perm.Units, u)
			}
		}
	}

	return
}

// IsUserRepoAdmin return true if user has admin right of a repo
func IsUserRepoAdmin(repo *Repository, user *User) (bool, error) {
	return isUserRepoAdmin(x, repo, user)
}

func isUserRepoAdmin(e Engine, repo *Repository, user *User) (bool, error) {
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
	if mode >= AccessModeAdmin {
		return true, nil
	}

	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return false, err
	}

	for _, team := range teams {
		if team.Authorize >= AccessModeAdmin {
			return true, nil
		}
	}
	return false, nil
}

// AccessLevel returns the Access a user has to a repository. Will return NoneAccess if the
// user does not have access.
func AccessLevel(user *User, repo *Repository) (AccessMode, error) {
	return accessLevelUnit(x, user, repo, UnitTypeCode)
}

// AccessLevelUnit returns the Access a user has to a repository's. Will return NoneAccess if the
// user does not have access.
func AccessLevelUnit(user *User, repo *Repository, unitType UnitType) (AccessMode, error) {
	return accessLevelUnit(x, user, repo, unitType)
}

func accessLevelUnit(e Engine, user *User, repo *Repository, unitType UnitType) (AccessMode, error) {
	perm, err := getUserRepoPermission(e, repo, user)
	if err != nil {
		return AccessModeNone, err
	}
	return perm.UnitAccessMode(unitType), nil
}

func hasAccessUnit(e Engine, user *User, repo *Repository, unitType UnitType, testMode AccessMode) (bool, error) {
	mode, err := accessLevelUnit(e, user, repo, unitType)
	return testMode <= mode, err
}

// HasAccessUnit returns ture if user has testMode to the unit of the repository
func HasAccessUnit(user *User, repo *Repository, unitType UnitType, testMode AccessMode) (bool, error) {
	return hasAccessUnit(x, user, repo, unitType, testMode)
}

// CanBeAssigned return true if user can be assigned to issue or pull requests in repo
// Currently any write access (code, issues or pr's) is assignable, to match assignee list in user interface.
// FIXME: user could send PullRequest also could be assigned???
func CanBeAssigned(user *User, repo *Repository, isPull bool) (bool, error) {
	if user.IsOrganization() {
		return false, fmt.Errorf("Organization can't be added as assignee [user_id: %d, repo_id: %d]", user.ID, repo.ID)
	}
	perm, err := GetUserRepoPermission(repo, user)
	if err != nil {
		return false, err
	}
	return perm.CanAccessAny(AccessModeWrite, UnitTypeCode, UnitTypeIssues, UnitTypePullRequests), nil
}

func hasAccess(e Engine, userID int64, repo *Repository) (bool, error) {
	var user *User
	var err error
	if userID > 0 {
		user, err = getUserByID(e, userID)
		if err != nil {
			return false, err
		}
	}
	perm, err := getUserRepoPermission(e, repo, user)
	if err != nil {
		return false, err
	}
	return perm.HasAccess(), nil
}

// HasAccess returns true if user has access to repo
func HasAccess(userID int64, repo *Repository) (bool, error) {
	return hasAccess(x, userID, repo)
}

// FilterOutRepoIdsWithoutUnitAccess filter out repos where user has no access to repositories
func FilterOutRepoIdsWithoutUnitAccess(u *User, repoIDs []int64, units ...UnitType) ([]int64, error) {
	i := 0
	for _, rID := range repoIDs {
		repo, err := GetRepositoryByID(rID)
		if err != nil {
			return nil, err
		}
		perm, err := GetUserRepoPermission(repo, u)
		if err != nil {
			return nil, err
		}
		if perm.CanReadAny(units...) {
			repoIDs[i] = rID
			i++
		}
	}
	return repoIDs[:i], nil
}
