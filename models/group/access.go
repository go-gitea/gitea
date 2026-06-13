// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	"gitea.dev/models/perm"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"xorm.io/builder"
)

type Capabilities struct {
	CanRead, CanWrite, CanAdmin, CanCreate bool
	IsMember, IsOwner                      bool
	HiddenByAncestor                       bool
}

func (g *Group) CanAccess(ctx context.Context, user *user_model.User) (bool, error) {
	return g.CanAccessAtLevel(ctx, user, perm.AccessModeRead)
}

func (g *Group) CanAccessAtLevel(ctx context.Context, user *user_model.User, level perm.AccessMode) (bool, error) {
	if user != nil {
		if user.IsAdmin {
			return true, nil
		}
		ownedBy, err := g.IsOwnedBy(ctx, user.ID)
		if err != nil {
			return false, err
		}
		if ownedBy {
			return true, nil
		}
		if level >= perm.AccessModeAdmin {
			return g.IsAdminOf(ctx, user.ID)
		}
		if level >= perm.AccessModeWrite {
			return g.CanCreateIn(ctx, user.ID)
		}
	}
	orCond := builder.Or(AccessibleGroupCondition(user))
	isMember, err := g.IsMemberOf(ctx, user)
	if err != nil {
		return false, err
	}
	if level == perm.AccessModeRead && !isMember {
		orCond = orCond.And(builder.Eq{"`repo_group`.visibility": structs.VisibleTypePublic})
	}
	return db.GetEngine(ctx).Table(g.TableName()).Where(builder.And(builder.Eq{"`repo_group`.id": g.ID}, orCond)).Exist()
}

func (g *Group) loadGroupMembershipWith(ctx context.Context, p perm.AccessMode, userID int64) (bool, error) {
	owner, err := user_model.GetUserByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	var org *org_model.Organization
	if owner.IsOrganization() {
		org = org_model.OrgFromUser(owner)
	}
	if org == nil {
		return g.OwnerID == userID, nil
	}
	switch p {
	case perm.AccessModeNone, perm.AccessModeRead, perm.AccessModeWrite:
		return org.IsOrgMember(ctx, userID)
	case perm.AccessModeAdmin:
		return org.IsOrgAdmin(ctx, userID)
	case perm.AccessModeOwner:
		return org.IsOwnedBy(ctx, userID)
	}
	return false, nil
}

func (g *Group) IsOwnedBy(ctx context.Context, userID int64) (bool, error) {
	return g.loadGroupMembershipWith(ctx, perm.AccessModeOwner, userID)
}

func (g *Group) IsMemberOf(ctx context.Context, user *user_model.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	return g.loadGroupMembershipWith(ctx, perm.AccessModeRead, user.ID)
}

func (g *Group) CanCreateIn(ctx context.Context, userID int64) (bool, error) {
	can, err := org_model.CanCreateOrgRepo(ctx, g.OwnerID, userID)
	if err != nil {
		return false, err
	}
	return can || g.OwnerID == userID, nil
}

func (g *Group) IsAdminOf(ctx context.Context, userID int64) (bool, error) {
	return g.loadGroupMembershipWith(ctx, perm.AccessModeAdmin, userID)
}

func (g *Group) IsPrivateBecauseOfParentPermissions(ctx context.Context, user *user_model.User) (bool, error) {
	accessibleSQL, accessibleArgs, err := builder.ToSQL(AccessibleGroupCondition(user))
	if err != nil {
		return false, err
	}

	var recursiveKeyword string
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = "RECURSIVE "
	}

	query := fmt.Sprintf(`WITH %s%s,
	group_access AS (
		SELECT COALESCE(MIN(CASE WHEN (%s) THEN 1 ELSE 0 END), 0) AS accessible
		FROM group_hierarchy repo_group
	)
	SELECT accessible FROM group_access`, recursiveKeyword, groupHierarchyCTEBuilder(
		builder.And(
			builder.Eq{
				"id": g.ID,
			},
			builder.Lt{
				"depth": NestingLimit,
			})), accessibleSQL)

	args := accessibleArgs
	var accessible int
	_, err = db.GetEngine(ctx).SQL(query, args...).Get(&accessible)
	return accessible == 0, err
}

func (g *Group) GetCapabilities(ctx context.Context, doer *user_model.User) (Capabilities, error) {
	var (
		caps Capabilities
		err  error
	)
	if err = g.LoadOwner(ctx); err != nil {
		return caps, err
	}

	if caps.HiddenByAncestor, err = g.IsPrivateBecauseOfParentPermissions(ctx, doer); err != nil {
		return caps, err
	}

	if caps.CanRead, err = g.CanAccessAtLevel(ctx, doer, perm.AccessModeRead); err != nil {
		return caps, err
	}
	if caps.CanWrite, err = g.CanAccessAtLevel(ctx, doer, perm.AccessModeWrite); err != nil {
		return caps, err
	}

	if caps.IsMember, err = g.IsMemberOf(ctx, doer); err != nil {
		return caps, err
	}

	if doer != nil {
		if caps.CanCreate, err = g.CanCreateIn(ctx, doer.ID); err != nil {
			return caps, err
		}
		var (
			isAdmin, isOwner bool
		)
		if isAdmin, err = g.IsAdminOf(ctx, doer.ID); err != nil {
			return caps, err
		}
		if isOwner, err = g.IsOwnedBy(ctx, doer.ID); err != nil {
			return caps, err
		}
		caps.CanAdmin = isAdmin || doer.IsAdmin
		caps.IsOwner = isOwner || doer.IsAdmin
	}

	caps.CanRead = caps.CanRead && (!caps.HiddenByAncestor || caps.IsMember)
	caps.CanWrite = caps.CanWrite && caps.CanRead

	if caps.CanAdmin || caps.IsOwner {
		caps.CanCreate = true
		caps.CanRead = true
		caps.CanWrite = true
		caps.IsMember = true
	}

	return caps, nil
}
