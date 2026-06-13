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
)

// commonCtx contains some common functions between APIContext and Context
type commonCtx interface {
	context.Context
	PathParamInt64(p string) int64
	PathParam(p string) string
	Written() bool
}

type RepoGroup struct {
	OwnerAsOrg   *organization.Organization
	Group        *group_model.Group
	GroupLink    string
	OrgGroupLink string
	capabilities group_model.Capabilities
}

func (g RepoGroup) Capabilities() group_model.Capabilities {
	return g.capabilities
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

func groupAssignment(ctx commonCtx, doer *user_model.User, _ bool, handleNotFound func(error), handleOtherError func(string, error), assign func(repoGroup *RepoGroup)) {
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

	if group.Owner == nil {
		err = group.LoadOwner(ctx)
		if err != nil {
			handleOtherError("LoadOwner", err)
			return
		}
	}
	if repoGroup.capabilities, err = group.GetCapabilities(ctx, doer); err != nil {
		handleOtherError("GetCapabilities", err)
		return
	}
	ownerAsOrg := organization.OrgFromUser(group.Owner)
	if group.Owner.IsOrganization() {
		repoGroup.OwnerAsOrg = ownerAsOrg
	}

	assign(repoGroup)
}

func GroupAssignmentWeb(args GroupAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		opts := args
		var err error
		groupAssignment(ctx, ctx.Doer, false, ctx.NotFound, ctx.ServerError, func(repoGroup *RepoGroup) {
			if ctx.Written() {
				return
			}

			canAccess := repoGroup.Capabilities().CanRead
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

			if (opts.RequireMember && !repoGroup.Capabilities().IsMember) ||
				(opts.RequireOwner && !repoGroup.Capabilities().IsOwner) {
				if ctx.Repo.Repository != nil {
					if !ctx.Repo.Permission.HasAnyUnitAccess() {
						ctx.NotFound(nil)
						return
					}
				} else {
					ctx.NotFound(nil)
					return
				}
			}

			ctx.Data["EnableFeed"] = setting.Other.EnableFeed
			ctx.Data["FeedURL"] = group.GroupLink()
			ctx.Data["IsOwnerOrg"] = group.Owner.IsOrganization()
			var isDoerOwner bool
			if ctx.IsSigned {
				if ctx.ContextUser != nil {
					isDoerOwner = ctx.ContextUser.ID == ctx.Doer.ID
				} else {
					isDoerOwner = repoGroup.Capabilities().IsOwner
				}
			}
			ctx.Data["IsDoerOwner"] = isDoerOwner
			ctx.Data["IsGroupOwner"] = repoGroup.Capabilities().IsOwner
			ctx.Data["IsGroupMember"] = repoGroup.Capabilities().IsMember
			ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
			ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
			ctx.Data["IsPublicMember"] = func(uid int64) bool {
				is, _ := organization.IsPublicMembership(ctx, ctx.Org.Organization.ID, uid)
				return is
			}
			ctx.Data["CanReadProjects"] = repoGroup.CanReadUnit(ctx, ctx.Doer, unit.TypeProjects)
			ctx.Data["CanCreateOrgRepo"] = repoGroup.Capabilities().CanCreate

			ctx.Data["IsGroupAdmin"] = repoGroup.Capabilities().CanAdmin
			if opts.RequireGroupAdmin && !repoGroup.Capabilities().CanAdmin {
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

			if err = AddGroupBreadcrumbs(ctx, group.ID); err != nil {
				ctx.ServerError("AddGroupBreadcrumbs", err)
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
		groupAssignment(ctx, ctx.Doer, true, func(err error) {
			ctx.APIErrorNotFound()
		}, func(str string, err error) {
			ctx.APIErrorInternal(fmt.Errorf("%s: %w", str, err))
		}, func(repoGroup *RepoGroup) {
			if ctx.Written() {
				return
			}

			group := repoGroup.Group
			if group.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
				ctx.APIErrorNotFound()
				return
			}

			if !repoGroup.Capabilities().CanRead && early404 {
				ctx.APIErrorNotFound()
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

func AddGroupBreadcrumbs(ctx *Context, gid int64) error {
	var err error
	ctx.Data["Breadcrumbs"], err = group_model.GetParentGroupChain(ctx, gid)
	if err != nil {
		return err
	}
	ctx.Data["CanAccessGroup"] = func(g *group_model.Group) bool {
		caps, err := g.GetCapabilities(ctx, ctx.Doer)
		if err != nil {
			return false
		}
		return caps.CanRead
	}

	return nil
}
