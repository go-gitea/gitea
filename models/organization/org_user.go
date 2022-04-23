// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
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

// IsOrganizationMember returns true if given user is member of organization.
func IsOrganizationMember(ctx context.Context, orgID, uid int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Table("org_user").
		Exist()
}

// IsPublicMembership returns true if the given user's membership of given org is public.
func IsPublicMembership(orgID, uid int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).
		Where("uid=?", uid).
		And("org_id=?", orgID).
		And("is_public=?", true).
		Table("org_user").
		Exist()
}

// CanCreateOrgRepo returns true if user can create repo in organization
func CanCreateOrgRepo(orgID, uid int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).
		Where(builder.Eq{"team.can_create_org_repo": true}).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		And("team_user.uid = ?", uid).
		And("team_user.org_id = ?", orgID).
		Exist(new(Team))
}
