// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// Permission contains all the permissions related variables to a repository for a user
type Permission struct {
	AccessMode AccessMode
	Units      []*RepoUnit
	UnitsMode  map[UnitType]AccessMode
}

// IsOwner returns true if current user is the owner of repository.
func (p *Permission) IsOwner() bool {
	return p.AccessMode >= AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (p *Permission) IsAdmin() bool {
	return p.AccessMode >= AccessModeAdmin
}

// HasAccess returns true if the current user has at least read access to any unit of this repository
func (p *Permission) HasAccess() bool {
	if p.UnitsMode == nil {
		return p.AccessMode >= AccessModeRead
	}
	return len(p.UnitsMode) > 0
}

// UnitAccessMode returns current user accessmode to the specify unit of the repository
func (p *Permission) UnitAccessMode(unitType UnitType) AccessMode {
	if p.UnitsMode == nil {
		for _, u := range p.Units {
			if u.Type == unitType {
				return p.AccessMode
			}
		}
		return AccessModeNone
	}
	return p.UnitsMode[unitType]
}

// CanAccess returns true if user has read access to the unit of the repository
func (p *Permission) CanAccess(unitType UnitType) bool {
	return p.UnitAccessMode(unitType) >= AccessModeRead
}

// CanAccessAny returns true if user has read access to any of the units of the repository
func (p *Permission) CanAccessAny(unitTypes ...UnitType) bool {
	for _, u := range unitTypes {
		if p.CanAccess(u) {
			return true
		}
	}
	return false
}

// CanWrite returns true if user could write to this unit
func (p *Permission) CanWrite(unitType UnitType) bool {
	return p.UnitAccessMode(unitType) >= AccessModeWrite
}

// CanWriteIssuesOrPulls returns true if isPull is true and user could write to pull requests and
// returns true if isPull is false and user could write to issues
func (p *Permission) CanWriteIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanWrite(UnitTypePullRequests)
	}
	return p.CanWrite(UnitTypeIssues)
}

// CanReadIssuesOrPulls returns true if isPull is true and user could read pull requests and
// returns true if isPull is false and user could read to issues
func (p *Permission) CanReadIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanAccess(UnitTypePullRequests)
	}
	return p.CanAccess(UnitTypeIssues)
}

// GetUserRepoPermission returns the user permissions to the repository
func GetUserRepoPermission(repo *Repository, user *User) (Permission, error) {
	return getUserRepoPermission(x, repo, user)
}

func getUserRepoPermission(e Engine, repo *Repository, user *User) (perm Permission, err error) {
	// anonymous user visit private repo.
	// TODO: anonymous user visit public unit of private repo???
	if user == nil && repo.IsPrivate {
		perm.AccessMode = AccessModeNone
		return
	}

	if err = repo.getUnits(e); err != nil {
		return
	}

	perm.Units = repo.Units

	// anonymous visit public repo
	if user == nil {
		perm.AccessMode = AccessModeRead
		return
	}

	// Admin or the owner has super access to the repository
	if user.IsAdmin || user.ID == repo.OwnerID {
		perm.AccessMode = AccessModeOwner
		return
	}

	// plain user
	perm.AccessMode, err = accessLevel(e, user.ID, repo)
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
	if isCollaborator, err := repo.isCollaborator(e, user.ID); err != nil {
		return perm, err
	} else if isCollaborator {
		for _, u := range repo.Units {
			perm.UnitsMode[u.Type] = perm.AccessMode
		}
	}

	// get units mode from teams
	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return
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

		// for a public repo on an organization, user have read permission on non-team defined units.
		if !found && !repo.IsPrivate {
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

// IsUserRepoAdmin return ture if user has admin right of a repo
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

	mode, err := accessLevel(e, user.ID, repo)
	if err != nil {
		return false, err
	}
	return mode >= AccessModeAdmin, nil
}
