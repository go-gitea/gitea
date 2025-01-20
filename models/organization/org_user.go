// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

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
}

func init() {
	db.RegisterModel(new(OrgUser))
}

// ErrUserHasOrgs represents a "UserHasOrgs" kind of error.
type ErrUserHasOrgs struct {
	UID int64
}

// IsErrUserHasOrgs checks if an error is a ErrUserHasOrgs.
func IsErrUserHasOrgs(err error) bool {
	_, ok := err.(ErrUserHasOrgs)
	return ok
}

func (err ErrUserHasOrgs) Error() string {
	return fmt.Sprintf("user still has membership of organizations [uid: %d]", err.UID)
}

// GetOrganizationCount returns count of membership of organization of the user.
func GetOrganizationCount(ctx context.Context, u *user_model.User) (int64, error) {
	return db.GetEngine(ctx).
		Where("uid=?", u.ID).
		Count(new(OrgUser))
}

// IsOrganizationOwner returns true if given user is in the owner team.
func IsOrganizationOwner(ctx context.Context, orgID, uid int64) (bool, error) {
	ownerTeam, err := GetOwnerTeam(ctx, orgID)
	if err != nil {
		if IsErrTeamNotExist(err) {
			log.Error("Organization does not have owner team: %d", orgID)
			return false, nil
		}
		return false, err
	}
	return IsTeamMember(ctx, orgID, ownerTeam.ID, uid)
}

// IsOrganizationAdmin returns true if given user is in the owner team or an admin team.
func IsOrganizationAdmin(ctx context.Context, orgID, uid int64) (bool, error) {
	teams, err := GetUserOrgTeams(ctx, orgID, uid)
	if err != nil {
		return false, err
	}
	for _, t := range teams {
		if t.AccessMode >= perm.AccessModeAdmin {
			return true, nil
		}
	}
	return false, nil
}

// IsOrganizationMember returns true if given user is member of organization.
func IsOrganizationMember(ctx context.Context, orgID, uid int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Table("org_user").
		Exist()
}

// IsPublicMembership returns true if the given user's membership of given org is public.
func IsPublicMembership(ctx context.Context, orgID, uid int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("uid=?", uid).
		And("org_id=?", orgID).
		And("is_public=?", true).
		Table("org_user").
		Exist()
}

// CanCreateOrgRepo returns true if user can create repo in organization
func CanCreateOrgRepo(ctx context.Context, orgID, uid int64) (bool, error) {
	return db.GetEngine(ctx).
		Where(builder.Eq{"team.can_create_org_repo": true}).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		And("team_user.uid = ?", uid).
		And("team_user.org_id = ?", orgID).
		Exist(new(Team))
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func IsUserOrgOwner(ctx context.Context, users user_model.UserList, orgID int64) map[int64]bool {
	results := make(map[int64]bool, len(users))
	for _, user := range users {
		results[user.ID] = false // Set default to false
	}
	ownerMaps, err := loadOrganizationOwners(ctx, users, orgID)
	if err == nil {
		for _, owner := range ownerMaps {
			results[owner.UID] = true
		}
	}
	return results
}

// GetOrgAssignees returns all users that have write access and can be assigned to issues
// of the any repository in the organization.
func GetOrgAssignees(ctx context.Context, orgID int64) (_ []*user_model.User, err error) {
	e := db.GetEngine(ctx)
	userIDs := make([]int64, 0, 10)
	if err = e.Table("access").
		Join("INNER", "repository", "`repository`.id = `access`.repo_id").
		Where("`repository`.owner_id = ? AND `access`.mode >= ?", orgID, perm.AccessModeWrite).
		Select("user_id").
		Find(&userIDs); err != nil {
		return nil, err
	}

	additionalUserIDs := make([]int64, 0, 10)
	if err = e.Table("team_user").
		Join("INNER", "team_repo", "`team_repo`.team_id = `team_user`.team_id").
		Join("INNER", "team_unit", "`team_unit`.team_id = `team_user`.team_id").
		Join("INNER", "repository", "`repository`.id = `team_repo`.repo_id").
		Where("`repository`.owner_id = ? AND (`team_unit`.access_mode >= ? OR (`team_unit`.access_mode = ? AND `team_unit`.`type` = ?))",
			orgID, perm.AccessModeWrite, perm.AccessModeRead, unit.TypePullRequests).
		Distinct("`team_user`.uid").
		Select("`team_user`.uid").
		Find(&additionalUserIDs); err != nil {
		return nil, err
	}

	uniqueUserIDs := make(container.Set[int64])
	uniqueUserIDs.AddMultiple(userIDs...)
	uniqueUserIDs.AddMultiple(additionalUserIDs...)

	users := make([]*user_model.User, 0, len(uniqueUserIDs))
	if len(userIDs) > 0 {
		if err = e.In("id", uniqueUserIDs.Values()).
			Where(builder.Eq{"`user`.is_active": true}).
			OrderBy(user_model.GetOrderByName()).
			Find(&users); err != nil {
			return nil, err
		}
	}

	return users, nil
}

func loadOrganizationOwners(ctx context.Context, users user_model.UserList, orgID int64) (map[int64]*TeamUser, error) {
	if len(users) == 0 {
		return nil, nil
	}
	ownerTeam, err := GetOwnerTeam(ctx, orgID)
	if err != nil {
		if IsErrTeamNotExist(err) {
			log.Error("Organization does not have owner team: %d", orgID)
			return nil, nil
		}
		return nil, err
	}

	userIDs := users.GetUserIDs()
	ownerMaps := make(map[int64]*TeamUser)
	err = db.GetEngine(ctx).In("uid", userIDs).
		And("org_id=?", orgID).
		And("team_id=?", ownerTeam.ID).
		Find(&ownerMaps)
	if err != nil {
		return nil, fmt.Errorf("find team users: %w", err)
	}
	return ownerMaps, nil
}
