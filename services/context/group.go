// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"strings"

	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	shared_group "code.gitea.io/gitea/models/shared/group"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
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
	Team                 *organization.Team
	Teams                []*organization.Team
	GroupTeam            *group_model.RepoGroupTeam
}

func (g *RepoGroup) CanWriteUnit(ctx context.Context, doer *user_model.User, unitType unit.Type) bool {
	return g.UnitPermission(ctx, doer, unitType) >= perm.AccessModeWrite
}

func (g *RepoGroup) CanReadUnit(ctx context.Context, doer *user_model.User, unitType unit.Type) bool {
	return g.UnitPermission(ctx, doer, unitType) >= perm.AccessModeRead
}

func (g *RepoGroup) UnitPermission(ctx context.Context, doer *user_model.User, unitType unit.Type) perm.AccessMode {
	if doer != nil {
		teams, err := organization.GetUserGroupTeams(ctx, g.Group.ID, doer.ID)
		if err != nil {
			log.Error("GetUserOrgTeams: %v", err)
			return perm.AccessModeNone
		}

		if err := teams.LoadUnits(ctx); err != nil {
			log.Error("LoadUnits: %v", err)
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

func groupAssignment(ctx commonCtx, doer *user_model.User, isSigned bool, handleNotFound func(error), handleOtherError func(string, error), assign func(repoGroup *RepoGroup, canAccess bool)) {
	var err error
	repoGroup := new(RepoGroup)
	if repoGroup.Group == nil {
		err = getGroupByParams(ctx, repoGroup, handleNotFound, handleOtherError)
	}
	if err != nil {
		handleOtherError("GetGroupByParams", err)
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
	ownerAsOrg := (*organization.Organization)(group.Owner)
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

	if isSigned && doer.IsAdmin {
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
			repoGroup.IsMember, err = shared_group.IsGroupMember(ctx, group.ID, doer)
			if err != nil {
				handleOtherError("IsOrgMember", err)
				return
			}
			repoGroup.CanCreateRepoOrGroup, err = group.CanCreateIn(ctx, doer.ID)
			if err != nil {
				handleOtherError("CanCreateIn", err)
				return
			}
		}
	}
	repoGroup.GroupLink = group.GroupLink()
	repoGroup.OrgGroupLink = group.OrgGroupLink()

	if repoGroup.IsMember {
		shouldSeeAllTeams := false
		if repoGroup.IsOwner {
			shouldSeeAllTeams = true
		} else {
			teams, err := organization.GetUserGroupTeams(ctx, group.ID, doer.ID)
			if err != nil {
				handleOtherError("GetUserTeams", err)
				return
			}
			for _, team := range teams {
				if team.IncludesAllRepositories && team.AccessMode >= perm.AccessModeAdmin {
					shouldSeeAllTeams = true
					break
				}
			}
		}
		if shouldSeeAllTeams {
			repoGroup.Teams, err = shared_group.GetGroupTeams(ctx, group.ID)
			if err != nil {
				handleOtherError("LoadTeams", err)
				return
			}
		} else {
			repoGroup.Teams, err = organization.GetUserGroupTeams(ctx, group.ID, doer.ID)
			if err != nil {
				handleOtherError("GetUserTeams", err)
				return
			}
		}
		// ctx.Data["NumTeams"] = len(repoGroup.Teams)
	}

	teamName := ctx.PathParam("team")
	if len(teamName) > 0 {
		teamExists := false
		for _, team := range repoGroup.Teams {
			if strings.EqualFold(team.LowerName, strings.ToLower(teamName)) {
				teamExists = true
				var groupTeam *group_model.RepoGroupTeam
				groupTeam, err = group_model.FindGroupTeamByTeamID(ctx, group.ID, team.ID)
				if err != nil {
					handleOtherError("FindGroupTeamByTeamID", err)
					return
				}
				repoGroup.GroupTeam = groupTeam
				repoGroup.Team = team
				repoGroup.IsMember = true
				break
			}
		}

		if !teamExists {
			handleNotFound(err)
			return
		}
		repoGroup.IsGroupAdmin = repoGroup.Team.IsOwnerTeam() || repoGroup.Team.AccessMode >= perm.AccessModeAdmin
	} else {
		for _, team := range repoGroup.Teams {
			if team.AccessMode >= perm.AccessModeAdmin {
				repoGroup.IsGroupAdmin = true
				break
			}
		}
	}
	if isSigned {
		isAdmin, err := group.IsAdminOf(ctx, doer.ID)
		if err != nil {
			handleOtherError("IsAdminOf", err)
			return
		}
		repoGroup.IsGroupAdmin = repoGroup.IsGroupAdmin || isAdmin
	}
	if !repoGroup.IsOwner && !repoGroup.IsGroupAdmin {
		canAccess = canAccess && !privateBecauseOfParent
	}
	assign(repoGroup, canAccess)
}

func GroupAssignmentWeb(args GroupAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		opts := args
		var err error
		groupAssignment(ctx, ctx.Doer, ctx.IsSigned, ctx.NotFound, ctx.ServerError, func(repoGroup *RepoGroup, canAccess bool) {
			if ctx.Written() {
				return
			}

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

			if (opts.RequireMember && !repoGroup.IsMember) ||
				(opts.RequireOwner && !repoGroup.IsOwner) {
				ctx.NotFound(nil)
				return
			}

			ctx.Data["EnableFeed"] = setting.Other.EnableFeed
			ctx.Data["FeedURL"] = group.GroupLink()
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
				ctx.Data["RenderedDescription"], err = markdown.RenderString(markup.NewRenderContext(ctx), group.Description)
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
			if repoGroup == nil {
				repoGroup = &RepoGroup{}
			}
			if !ctx.IsSigned {
				ctx.Data["SignedUser"] = &user_model.User{}
			}
			if repoGroup.IsMember {
				ctx.Data["NumTeams"] = len(repoGroup.Teams)
			}
			if repoGroup.Team != nil {
				ctx.Data["Team"] = repoGroup.Team
				ctx.Data["IsTeamMember"] = repoGroup.IsMember
			}
			ctx.RepoGroup = repoGroup
		})
	}
}

func GroupAssignmentAPI() func(ctx *APIContext) {
	return func(ctx *APIContext) {
		groupAssignment(ctx, ctx.Doer, ctx.IsSigned, func(err error) {
			ctx.APIErrorNotFound(err)
		}, func(_ string, err error) {
			ctx.APIErrorInternal(err)
		}, func(repoGroup *RepoGroup, canAccess bool) {
			if ctx.Written() {
				return
			}

			group := repoGroup.Group
			if group.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
				ctx.APIErrorNotFound(nil)
				return
			}
			if ctx.IsSigned {
				if !canAccess && group.Visibility != structs.VisibleTypePublic {
					ctx.APIErrorNotFound(nil)
					return
				}
			}
			if !canAccess {
				ctx.APIErrorNotFound(nil)
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
