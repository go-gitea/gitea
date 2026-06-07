// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"

	group_model "gitea.dev/models/group"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// commonCtx contains some common functions between APIContext and Context
type commonCtx interface {
	context.Context
	PathParamInt64(p string) int64
	PathParam(p string) string
	Written() bool
}

type RepoGroup struct {
	IsOwner              bool
	IsMember             bool
	IsGroupAdmin         bool
	Group                *group_model.Group
	GroupLink            string
	OrgGroupLink         string
	CanCreateRepoOrGroup bool
	doerCanAccess        bool
}

func (g *RepoGroup) DoerCanAccess() bool {
	return g.doerCanAccess
}

func (g *RepoGroup) CanWriteUnit(ctx context.Context, doer *user_model.User, unitType unit.Type) bool {
	return g.UnitPermission(ctx, doer, unitType) >= perm.AccessModeWrite
}

func (g *RepoGroup) CanReadUnit(ctx context.Context, doer *user_model.User, unitType unit.Type) bool {
	return g.UnitPermission(ctx, doer, unitType) >= perm.AccessModeRead
}

func (g *RepoGroup) UnitPermission(ctx context.Context, doer *user_model.User, unitType unit.Type) perm.AccessMode {
	if doer != nil {
		teams, err := organization.GetUserOrgTeams(ctx, g.Group.OwnerID, doer.ID)
		if err != nil {
			return perm.AccessModeNone
		}

		if err := teams.LoadUnits(ctx); err != nil {
			return perm.AccessModeNone
		}

		if len(teams) > 0 {
			return teams.UnitMaxAccess(unitType)
		}
	}

	if g.Group.Visibility.IsPublic() {
		return perm.AccessModeRead
	}

	return perm.AccessModeNone
}

func getGroupByParams(ctx commonCtx, repoGroup *RepoGroup, handleNotFound func(error), handleOtherError func(string, error)) (err error) {
	groupID := ctx.PathParamInt64("group_id")

	repoGroup.Group, err = group_model.GetGroupByID(ctx, groupID)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			handleNotFound(err)
		} else {
			handleOtherError("GetGroupByID", err)
		}
		return err
	}
	if err = repoGroup.Group.LoadAttributes(ctx); err != nil {
		handleOtherError("LoadAttributes", err)
	}
	return err
}

func GetGroupByParams(ctx *Context) (err error) {
	if ctx.RepoGroup == nil {
		ctx.RepoGroup = &RepoGroup{}
	}
	return getGroupByParams(ctx, ctx.RepoGroup, ctx.NotFound, ctx.ServerError)
}

type GroupAssignmentOptions struct {
	RequireMember     bool
	RequireOwner      bool
	RequireGroupAdmin bool
}

func groupAssignment(ctx commonCtx, doer *user_model.User, isSigned, _ bool, handleNotFound func(error), handleOtherError func(string, error), assign func(repoGroup *RepoGroup)) {
	var err error
	repoGroup := new(RepoGroup)
	err = getGroupByParams(ctx, repoGroup, handleNotFound, handleOtherError)
	if err != nil {
		return
	}
	if ctx.Written() {
		return
	}
	group := repoGroup.Group
	canAccess, err := group.CanAccess(ctx, doer)
	if err != nil {
		handleOtherError("error checking group access", err)
		return
	}
	privateBecauseOfParent, err := group.IsPrivateBecauseOfParentPermissions(ctx, doer)
	if err != nil {
		handleOtherError("error checking group access", err)
		return
	}
	if group.Owner == nil {
		err = group.LoadOwner(ctx)
		if err != nil {
			handleOtherError("LoadOwner", err)
			return
		}
	}
	ownerAsOrg := organization.OrgFromUser(group.Owner)
	var orgWideAdmin, orgWideOwner, isOwnedBy bool

	if isSigned {
		if orgWideAdmin, err = ownerAsOrg.IsOrgAdmin(ctx, doer.ID); err != nil {
			handleOtherError("IsOrgAdmin", err)
			return
		}
		if orgWideOwner, err = ownerAsOrg.IsOwnedBy(ctx, doer.ID); err != nil {
			handleOtherError("IsOwnedBy", err)
			return
		}
	}
	if orgWideOwner {
		repoGroup.IsOwner = true
	}
	if orgWideAdmin {
		repoGroup.IsGroupAdmin = true
	}

	if isSigned && (doer.IsAdmin || doer.ID == group.OwnerID) {
		repoGroup.IsOwner = true
		repoGroup.IsMember = true
		repoGroup.IsGroupAdmin = true
		repoGroup.CanCreateRepoOrGroup = true
	} else if isSigned {
		isOwnedBy, err = group.IsOwnedBy(ctx, doer.ID)
		if err != nil {
			handleOtherError("IsOwnedBy", err)
			return
		}
		repoGroup.IsOwner = repoGroup.IsOwner || isOwnedBy

		if repoGroup.IsOwner {
			repoGroup.IsMember = true
			repoGroup.IsGroupAdmin = true
			repoGroup.CanCreateRepoOrGroup = true
		} else {
			repoGroup.IsMember, err = group.IsMemberOf(ctx, doer)
			if err != nil {
				handleOtherError("IsOrgMember", err)
				return
			}
			repoGroup.CanCreateRepoOrGroup, err = group.CanCreateIn(ctx, doer.ID)
			if err != nil {
				handleOtherError("CanCreateIn", err)
				return
			}
			repoGroup.IsGroupAdmin, err = group.IsAdminOf(ctx, doer.ID)
			if err != nil {
				handleOtherError("IsAdminOf", err)
				return
			}
		}
	}
	repoGroup.GroupLink = group.GroupLink()
	repoGroup.OrgGroupLink = util.Iif(group.Owner.IsOrganization(), group.OrgGroupLink(), group.UserGroupLink())
	if !repoGroup.IsOwner && !repoGroup.IsGroupAdmin {
		canAccess = canAccess && !privateBecauseOfParent
	}
	repoGroup.doerCanAccess = canAccess
	assign(repoGroup)
}

func GroupAssignmentWeb(args GroupAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		opts := args
		var err error
		groupAssignment(ctx, ctx.Doer, ctx.IsSigned, false, ctx.NotFound, ctx.ServerError, func(repoGroup *RepoGroup) {
			if ctx.Written() {
				return
			}

			canAccess := repoGroup.doerCanAccess
			group := repoGroup.Group
			if group.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
				ctx.NotFound(nil)
				return
			}

			if group.Visibility == structs.VisibleTypePrivate {
				opts.RequireMember = true
			} else if !canAccess && group.Visibility != structs.VisibleTypePublic {
				ctx.NotFound(nil)
				return
			}

			if ((opts.RequireMember && !repoGroup.IsMember) ||
				(opts.RequireOwner && !repoGroup.IsOwner)) &&
				(ctx.Repo.Repository != nil &&
					!ctx.Repo.Permission.HasAnyUnitAccessOrPublicAccess()) {
				ctx.NotFound(nil)
				return
			}

			ctx.Data["EnableFeed"] = setting.Other.EnableFeed
			ctx.Data["FeedURL"] = group.GroupLink()
			ctx.Data["IsOwnerOrg"] = group.Owner.IsOrganization()
			var isDoerOwner bool
			if ctx.IsSigned {
				if ctx.ContextUser != nil {
					isDoerOwner = ctx.ContextUser.ID == ctx.Doer.ID
				} else {
					isDoerOwner = repoGroup.IsOwner
				}
			}
			ctx.Data["IsDoerOwner"] = isDoerOwner
			ctx.Data["IsGroupOwner"] = repoGroup.IsOwner
			ctx.Data["IsGroupMember"] = repoGroup.IsMember
			ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
			ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
			ctx.Data["IsPublicMember"] = func(uid int64) bool {
				is, _ := organization.IsPublicMembership(ctx, ctx.Org.Organization.ID, uid)
				return is
			}
			ctx.Data["CanReadProjects"] = repoGroup.CanReadUnit(ctx, ctx.Doer, unit.TypeProjects)
			ctx.Data["CanCreateOrgRepo"] = repoGroup.CanCreateRepoOrGroup

			ctx.Data["IsGroupAdmin"] = repoGroup.IsGroupAdmin
			if opts.RequireGroupAdmin && !repoGroup.IsGroupAdmin {
				ctx.NotFound(nil)
				return
			}

			if len(group.Description) != 0 {
				ctx.Data["RenderedGroupDescription"], err = markdown.RenderString(markup.NewRenderContext(ctx), group.Description)
				if err != nil {
					ctx.ServerError("RenderString", err)
					return
				}
			}
			ctx.Data["Group"] = group
			ctx.Data["ContextGroup"] = repoGroup
			ctx.Data["Doer"] = ctx.Doer
			ctx.Data["GroupLink"] = group.GroupLink()
			ctx.Data["OrgGroupLink"] = repoGroup.OrgGroupLink
			ctx.Data["Breadcrumbs"], err = group_model.GetParentGroupChain(ctx, group.ID)
			if err != nil {
				ctx.ServerError("GetParentGroupChain", err)
				return
			}
			if !ctx.IsSigned {
				ctx.Data["SignedUser"] = &user_model.User{}
			}
			ctx.RepoGroup = repoGroup
		})
	}
}

func GroupAssignmentAPI(early404 bool) func(ctx *APIContext) {
	return func(ctx *APIContext) {
		groupAssignment(ctx, ctx.Doer, ctx.IsSigned, true, func(err error) {
			ctx.APIErrorNotFound(err)
		}, func(str string, err error) {
			ctx.APIErrorInternal(fmt.Errorf("%s: %w", str, err))
		}, func(repoGroup *RepoGroup) {
			if ctx.Written() {
				return
			}

			canAccess := repoGroup.doerCanAccess
			group := repoGroup.Group
			if group.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
				ctx.APIErrorNotFound(nil)
				return
			}

			if !canAccess && early404 {
				ctx.APIErrorNotFound(nil)
				return
			}
			ctx.RepoGroup = repoGroup
		})
	}
}

func groupIsCurrent(ctx *Context) func(groupID int64) bool { //nolint:unused // will be used later
	return func(groupID int64) bool {
		if ctx.RepoGroup.Group == nil {
			return false
		}
		return ctx.RepoGroup.Group.ID == groupID
	}
}
