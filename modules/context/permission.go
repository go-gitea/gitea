// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import "code.gitea.io/gitea/models"

// Permission contains all the permissions related variables to a repository
type Permission struct {
	AccessMode models.AccessMode
	Units      []*models.RepoUnit
	UnitsMode  map[models.UnitType]models.AccessMode
}

// IsOwner returns true if current user is the owner of repository.
func (p *Permission) IsOwner() bool {
	return p.AccessMode >= models.AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (p *Permission) IsAdmin() bool {
	return p.AccessMode >= models.AccessModeAdmin
}

// HasAccess returns true if the current user has at least read access to any unit of this repository
func (p *Permission) HasAccess() bool {
	if p.UnitsMode == nil {
		return p.AccessMode >= models.AccessModeRead
	}
	return len(p.UnitsMode) > 0
}

// UnitAccessMode returns current user accessmode to the specify unit of the repository
func (p *Permission) UnitAccessMode(unitType models.UnitType) models.AccessMode {
	if p.UnitsMode == nil {
		return p.AccessMode
	}
	return p.UnitsMode[unitType]
}

// CanAccess returns true if user has read access to the unit of the repository
func (p *Permission) CanAccess(unitType models.UnitType) bool {
	return p.UnitAccessMode(unitType) >= models.AccessModeRead
}

// CanWrite returns true if user could write to this unit
func (p *Permission) CanWrite(unitType models.UnitType) bool {
	return p.UnitAccessMode(unitType) >= models.AccessModeWrite
}

// CanWriteIssuesOrPulls returns true if isPull is true and user could write to pull requests and
// returns true if isPull is false and user could write to issues
func (p *Permission) CanWriteIssuesOrPulls(isPull bool) bool {
	if isPull {
		return p.CanWrite(models.UnitTypePullRequests)
	}
	return p.CanWrite(models.UnitTypeIssues)
}
