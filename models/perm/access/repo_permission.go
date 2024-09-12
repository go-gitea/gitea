// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"context"
	"fmt"
	"slices"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// Permission contains all the permissions related variables to a repository for a user
type Permission struct {
	AccessMode perm_model.AccessMode

	units     []*repo_model.RepoUnit
	unitsMode map[unit.Type]perm_model.AccessMode

	everyoneAccessMode map[unit.Type]perm_model.AccessMode
}

// IsOwner returns true if current user is the owner of repository.
func (p *Permission) IsOwner() bool {
	return p.AccessMode >= perm_model.AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (p *Permission) IsAdmin() bool {
	return p.AccessMode >= perm_model.AccessModeAdmin
}

// HasAnyUnitAccess returns true if the user might have at least one access mode to any unit of this repository.
// It doesn't count the "everyone access mode".
func (p *Permission) HasAnyUnitAccess() bool {
	for _, v := range p.unitsMode {
		if v >= perm_model.AccessModeRead {
			return true
		}
	}
	return p.AccessMode >= perm_model.AccessModeRead
}

func (p *Permission) HasAnyUnitAccessOrEveryoneAccess() bool {
	for _, v := range p.everyoneAccessMode {
		if v >= perm_model.AccessModeRead {
			return true
		}
	}
	return p.HasAnyUnitAccess()
}

// HasUnits returns true if the permission contains attached units
func (p *Permission) HasUnits() bool {
	return len(p.units) > 0
}

// GetFirstUnitRepoID returns the repo ID of the first unit, it is a fragile design and should NOT be used anymore
// deprecated
func (p *Permission) GetFirstUnitRepoID() int64 {
	if len(p.units) > 0 {
		return p.units[0].RepoID
	}
	return 0
}

// UnitAccessMode returns current user access mode to the specify unit of the repository
// It also considers "everyone access mode"
func (p *Permission) UnitAccessMode(unitType unit.Type) perm_model.AccessMode {
	// if the units map contains the access mode, use it, but admin/owner mode could override it
	if m, ok := p.unitsMode[unitType]; ok {
		return util.Iif(p.AccessMode >= perm_model.AccessModeAdmin, p.AccessMode, m)
	}
	// if the units map does not contain the access mode, return the default access mode if the unit exists
	unitDefaultAccessMode := max(p.AccessMode, p.everyoneAccessMode[unitType])
	hasUnit := slices.ContainsFunc(p.units, func(u *repo_model.RepoUnit) bool { return u.Type == unitType })
	return util.Iif(hasUnit, unitDefaultAccessMode, perm_model.AccessModeNone)
}

func (p *Permission) SetUnitsWithDefaultAccessMode(units []*repo_model.RepoUnit, mode perm_model.AccessMode) {
	p.units = units
	p.unitsMode = make(map[unit.Type]perm_model.AccessMode)
	for _, u := range p.units {
		p.unitsMode[u.Type] = mode
	}
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

func (p *Permission) ReadableUnitTypes() []unit.Type {
	types := make([]unit.Type, 0, len(p.units))
	for _, u := range p.units {
		if p.CanRead(u.Type) {
			types = append(types, u.Type)
		}
	}
	return types
}

func (p *Permission) LogString() string {
	format := "<Permission AccessMode=%s, %d Units, %d UnitsMode(s): [ "
	args := []any{p.AccessMode.ToString(), len(p.units), len(p.unitsMode)}

	for i, u := range p.units {
		config := ""
		if u.Config != nil {
			configBytes, err := u.Config.ToDB()
			config = string(configBytes)
			if err != nil {
				config = err.Error()
			}
		}
		format += "\nUnits[%d]: ID: %d RepoID: %d Type: %s Config: %s"
		args = append(args, i, u.ID, u.RepoID, u.Type.LogString(), config)
	}
	for key, value := range p.unitsMode {
		format += "\nUnitMode[%-v]: %-v"
		args = append(args, key.LogString(), value.LogString())
	}
	format += " ]>"
	return fmt.Sprintf(format, args...)
}

func applyEveryoneRepoPermission(user *user_model.User, perm *Permission) {
	if user == nil || user.ID <= 0 {
		return
	}
	for _, u := range perm.units {
		if u.EveryoneAccessMode >= perm_model.AccessModeRead && u.EveryoneAccessMode > perm.everyoneAccessMode[u.Type] {
			if perm.everyoneAccessMode == nil {
				perm.everyoneAccessMode = make(map[unit.Type]perm_model.AccessMode)
			}
			perm.everyoneAccessMode[u.Type] = u.EveryoneAccessMode
		}
	}
}

// GetUserRepoPermission returns the user permissions to the repository
func GetUserRepoPermission(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (perm Permission, err error) {
	defer func() {
		if err == nil {
			applyEveryoneRepoPermission(user, &perm)
		}
		if log.IsTrace() {
			log.Trace("Permission Loaded for user %-v in repo %-v, permissions: %-+v", user, repo, perm)
		}
	}()

	if err = repo.LoadUnits(ctx); err != nil {
		return perm, err
	}
	perm.units = repo.Units

	// anonymous user visit private repo.
	// TODO: anonymous user visit public unit of private repo???
	if user == nil && repo.IsPrivate {
		perm.AccessMode = perm_model.AccessModeNone
		return perm, nil
	}

	var isCollaborator bool
	if user != nil {
		isCollaborator, err = repo_model.IsCollaborator(ctx, repo.ID, user.ID)
		if err != nil {
			return perm, err
		}
	}

	if err = repo.LoadOwner(ctx); err != nil {
		return perm, err
	}

	// Prevent strangers from checking out public repo of private organization/users
	// Allow user if they are collaborator of a repo within a private user or a private organization but not a member of the organization itself
	if !organization.HasOrgOrUserVisible(ctx, repo.Owner, user) && !isCollaborator {
		perm.AccessMode = perm_model.AccessModeNone
		return perm, nil
	}

	// anonymous visit public repo
	if user == nil {
		perm.AccessMode = perm_model.AccessModeRead
		return perm, nil
	}

	// Admin or the owner has super access to the repository
	if user.IsAdmin || user.ID == repo.OwnerID {
		perm.AccessMode = perm_model.AccessModeOwner
		return perm, nil
	}

	// plain user
	perm.AccessMode, err = accessLevel(ctx, user, repo)
	if err != nil {
		return perm, err
	}

	if !repo.Owner.IsOrganization() {
		return perm, nil
	}

	perm.unitsMode = make(map[unit.Type]perm_model.AccessMode)

	// Collaborators on organization
	if isCollaborator {
		for _, u := range repo.Units {
			perm.unitsMode[u.Type] = perm.AccessMode
		}
	}

	// get units mode from teams
	teams, err := organization.GetUserRepoTeams(ctx, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return perm, err
	}

	// if user in an owner team
	for _, team := range teams {
		if team.AccessMode >= perm_model.AccessModeAdmin {
			perm.AccessMode = perm_model.AccessModeOwner
			perm.unitsMode = nil
			return perm, nil
		}
	}

	for _, u := range repo.Units {
		var found bool
		for _, team := range teams {
			if teamMode, exist := team.UnitAccessModeEx(ctx, u.Type); exist {
				perm.unitsMode[u.Type] = max(perm.unitsMode[u.Type], teamMode)
				found = true
			}
		}

		// for a public repo on an organization, a non-restricted user has read permission on non-team defined units.
		if !found && !repo.IsPrivate && !user.IsRestricted {
			if _, ok := perm.unitsMode[u.Type]; !ok {
				perm.unitsMode[u.Type] = perm_model.AccessModeRead
			}
		}
	}

	// remove no permission units
	perm.units = make([]*repo_model.RepoUnit, 0, len(repo.Units))
	for t := range perm.unitsMode {
		for _, u := range repo.Units {
			if u.Type == t {
				perm.units = append(perm.units, u)
			}
		}
	}

	return perm, err
}

// IsUserRealRepoAdmin check if this user is real repo admin
func IsUserRealRepoAdmin(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if repo.OwnerID == user.ID {
		return true, nil
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return false, err
	}

	accessMode, err := accessLevel(ctx, user, repo)
	if err != nil {
		return false, err
	}

	return accessMode >= perm_model.AccessModeAdmin, nil
}

// IsUserRepoAdmin return true if user has admin right of a repo
func IsUserRepoAdmin(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if user == nil || repo == nil {
		return false, nil
	}
	if user.IsAdmin {
		return true, nil
	}

	mode, err := accessLevel(ctx, user, repo)
	if err != nil {
		return false, err
	}
	if mode >= perm_model.AccessModeAdmin {
		return true, nil
	}

	teams, err := organization.GetUserRepoTeams(ctx, repo.OwnerID, user.ID, repo.ID)
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
func AccessLevel(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (perm_model.AccessMode, error) { //nolint
	return AccessLevelUnit(ctx, user, repo, unit.TypeCode)
}

// AccessLevelUnit returns the Access a user has to a repository's. Will return NoneAccess if the
// user does not have access.
func AccessLevelUnit(ctx context.Context, user *user_model.User, repo *repo_model.Repository, unitType unit.Type) (perm_model.AccessMode, error) { //nolint
	perm, err := GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		return perm_model.AccessModeNone, err
	}
	return perm.UnitAccessMode(unitType), nil
}

// HasAccessUnit returns true if user has testMode to the unit of the repository
func HasAccessUnit(ctx context.Context, user *user_model.User, repo *repo_model.Repository, unitType unit.Type, testMode perm_model.AccessMode) (bool, error) {
	mode, err := AccessLevelUnit(ctx, user, repo, unitType)
	return testMode <= mode, err
}

// CanBeAssigned return true if user can be assigned to issue or pull requests in repo
// Currently any write access (code, issues or pr's) is assignable, to match assignee list in user interface.
func CanBeAssigned(ctx context.Context, user *user_model.User, repo *repo_model.Repository, _ bool) (bool, error) {
	if user.IsOrganization() {
		return false, fmt.Errorf("organization can't be added as assignee [user_id: %d, repo_id: %d]", user.ID, repo.ID)
	}
	perm, err := GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return perm.CanAccessAny(perm_model.AccessModeWrite, unit.AllRepoUnitTypes...) ||
		perm.CanAccessAny(perm_model.AccessModeRead, unit.TypePullRequests), nil
}

// HasAnyUnitAccess see the comment of "perm.HasAnyUnitAccess"
func HasAnyUnitAccess(ctx context.Context, userID int64, repo *repo_model.Repository) (bool, error) {
	var user *user_model.User
	var err error
	if userID > 0 {
		user, err = user_model.GetUserByID(ctx, userID)
		if err != nil {
			return false, err
		}
	}
	perm, err := GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return perm.HasAnyUnitAccess(), nil
}

// getUsersWithAccessMode returns users that have at least given access mode to the repository.
func getUsersWithAccessMode(ctx context.Context, repo *repo_model.Repository, mode perm_model.AccessMode) (_ []*user_model.User, err error) {
	if err = repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	accesses := make([]*Access, 0, 10)
	if err = e.Where("repo_id = ? AND mode >= ?", repo.ID, mode).Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(accesses)+1)
	if len(accesses) > 0 {
		userIDs := make([]int64, len(accesses))
		for i := 0; i < len(accesses); i++ {
			userIDs[i] = accesses[i].UserID
		}

		if err = e.In("id", userIDs).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetRepoReaders returns all users that have explicit read access or higher to the repository.
func GetRepoReaders(ctx context.Context, repo *repo_model.Repository) (_ []*user_model.User, err error) {
	return getUsersWithAccessMode(ctx, repo, perm_model.AccessModeRead)
}

// GetRepoWriters returns all users that have write access to the repository.
func GetRepoWriters(ctx context.Context, repo *repo_model.Repository) (_ []*user_model.User, err error) {
	return getUsersWithAccessMode(ctx, repo, perm_model.AccessModeWrite)
}

// IsRepoReader returns true if user has explicit read access or higher to the repository.
func IsRepoReader(ctx context.Context, repo *repo_model.Repository, userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	return db.GetEngine(ctx).Where("repo_id = ? AND user_id = ? AND mode >= ?", repo.ID, userID, perm_model.AccessModeRead).Get(&Access{})
}

// CheckRepoUnitUser check whether user could visit the unit of this repository
func CheckRepoUnitUser(ctx context.Context, repo *repo_model.Repository, user *user_model.User, unitType unit.Type) bool {
	if user != nil && user.IsAdmin {
		return true
	}
	perm, err := GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		log.Error("GetUserRepoPermission: %w", err)
		return false
	}

	return perm.CanRead(unitType)
}
