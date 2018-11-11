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
		return p.AccessMode
	}
	return p.UnitsMode[unitType]
}

// CanAccess returns true if user has read access to the unit of the repository
func (p *Permission) CanAccess(unitType UnitType) bool {
	return p.UnitAccessMode(unitType) >= AccessModeRead
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

// GetUserRepoPermission returns the user permissions to the repository
func GetUserRepoPermission(repo *Repository, user *User) (Permission, error) {
	return getUserRepoPermission(x, repo, user)
}

func getUserRepoPermission(e Engine, repo *Repository, user *User) (perm Permission, err error) {
	// anonymous user visit private repo. TODO: anonymous user visit public unit of private repo???
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

	// Admin has super access or user is the owner of the repository
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

	teams, err := getUserRepoTeams(e, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return
	}

	perm.UnitsMode = make(map[UnitType]AccessMode)
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

		if !found && !repo.IsPrivate {
			perm.UnitsMode[u.Type] = AccessModeRead
		}
	}

	perm.Units = make([]*RepoUnit, 0, len(repo.Units))
	for t, _ := range perm.UnitsMode {
		for _, u := range repo.Units {
			if u.Type == t {
				perm.Units = append(perm.Units, u)
			}
		}
	}

	return
}
