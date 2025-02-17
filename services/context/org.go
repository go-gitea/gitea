// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package context

import (
	"strings"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// Organization contains organization context
type Organization struct {
	IsOwner          bool
	IsMember         bool
	IsTeamMember     bool // Is member of team.
	IsTeamAdmin      bool // In owner team or team that has admin permission level.
	Organization     *organization.Organization
	OrgLink          string
	CanCreateOrgRepo bool

	Team  *organization.Team
	Teams []*organization.Team
}

func (org *Organization) CanWriteUnit(ctx *Context, unitType unit.Type) bool {
	return org.Organization.UnitPermission(ctx, ctx.Doer, unitType) >= perm.AccessModeWrite
}

func (org *Organization) CanReadUnit(ctx *Context, unitType unit.Type) bool {
	return org.Organization.UnitPermission(ctx, ctx.Doer, unitType) >= perm.AccessModeRead
}

func GetOrganizationByParams(ctx *Context) {
	orgName := ctx.PathParam("org")

	var err error

	ctx.Org.Organization, err = organization.GetOrgByName(ctx, orgName)
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			redirectUserID, err := user_model.LookupUserRedirect(ctx, orgName)
			if err == nil {
				RedirectToUser(ctx.Base, orgName, redirectUserID)
			} else if user_model.IsErrUserRedirectNotExist(err) {
				ctx.NotFound(err)
			} else {
				ctx.ServerError("LookupUserRedirect", err)
			}
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}
}

type OrgAssignmentOptions struct {
	RequireMember     bool
	RequireOwner      bool
	RequireTeamMember bool
	RequireTeamAdmin  bool
}

// OrgAssignment returns a middleware to handle organization assignment
func OrgAssignment(opts OrgAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		var err error
		if ctx.ContextUser == nil {
			// if Organization is not defined, get it from params
			if ctx.Org.Organization == nil {
				GetOrganizationByParams(ctx)
				if ctx.Written() {
					return
				}
			}
		} else if ctx.ContextUser.IsOrganization() {
			ctx.Org.Organization = (*organization.Organization)(ctx.ContextUser)
		} else {
			// ContextUser is an individual User
			return
		}

		org := ctx.Org.Organization

		// Handle Visibility
		if org.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
			// We must be signed in to see limited or private organizations
			ctx.NotFound(err)
			return
		}

		if org.Visibility == structs.VisibleTypePrivate {
			opts.RequireMember = true
		} else if ctx.IsSigned && ctx.Doer.IsRestricted {
			opts.RequireMember = true
		}

		ctx.ContextUser = org.AsUser()
		ctx.Data["Org"] = org

		// Admin has super access.
		if ctx.IsSigned && ctx.Doer.IsAdmin {
			ctx.Org.IsOwner = true
			ctx.Org.IsMember = true
			ctx.Org.IsTeamMember = true
			ctx.Org.IsTeamAdmin = true
			ctx.Org.CanCreateOrgRepo = true
		} else if ctx.IsSigned {
			ctx.Org.IsOwner, err = org.IsOwnedBy(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("IsOwnedBy", err)
				return
			}

			if ctx.Org.IsOwner {
				ctx.Org.IsMember = true
				ctx.Org.IsTeamMember = true
				ctx.Org.IsTeamAdmin = true
				ctx.Org.CanCreateOrgRepo = true
			} else {
				ctx.Org.IsMember, err = org.IsOrgMember(ctx, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("IsOrgMember", err)
					return
				}
				ctx.Org.CanCreateOrgRepo, err = org.CanCreateOrgRepo(ctx, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("CanCreateOrgRepo", err)
					return
				}
			}
		} else {
			// Fake data.
			ctx.Data["SignedUser"] = &user_model.User{}
		}
		if (opts.RequireMember && !ctx.Org.IsMember) || (opts.RequireOwner && !ctx.Org.IsOwner) {
			ctx.NotFound(err)
			return
		}
		ctx.Data["IsOrganizationOwner"] = ctx.Org.IsOwner
		ctx.Data["IsOrganizationMember"] = ctx.Org.IsMember
		ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
		ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
		ctx.Data["IsPublicMember"] = func(uid int64) bool {
			is, _ := organization.IsPublicMembership(ctx, ctx.Org.Organization.ID, uid)
			return is
		}
		ctx.Data["CanCreateOrgRepo"] = ctx.Org.CanCreateOrgRepo

		ctx.Org.OrgLink = org.AsUser().OrganisationLink()
		ctx.Data["OrgLink"] = ctx.Org.OrgLink

		// Member
		findMembersOpts := &organization.FindOrgMembersOpts{
			Doer:         ctx.Doer,
			OrgID:        org.ID,
			IsDoerMember: ctx.Org.IsMember,
		}
		ctx.Data["NumMembers"], err = organization.CountOrgMembers(ctx, findMembersOpts)
		if err != nil {
			ctx.ServerError("CountOrgMembers", err)
			return
		}

		// Team.
		if ctx.Org.IsMember {
			shouldSeeAllTeams := false
			if ctx.Org.IsOwner {
				shouldSeeAllTeams = true
			} else {
				teams, err := org.GetUserTeams(ctx, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("GetUserTeams", err)
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
				ctx.Org.Teams, err = org.LoadTeams(ctx)
				if err != nil {
					ctx.ServerError("LoadTeams", err)
					return
				}
			} else {
				ctx.Org.Teams, err = org.GetUserTeams(ctx, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("GetUserTeams", err)
					return
				}
			}
			ctx.Data["NumTeams"] = len(ctx.Org.Teams)
		}

		teamName := ctx.PathParam("team")
		if len(teamName) > 0 {
			teamExists := false
			for _, team := range ctx.Org.Teams {
				if team.LowerName == strings.ToLower(teamName) {
					teamExists = true
					ctx.Org.Team = team
					ctx.Org.IsTeamMember = true
					ctx.Data["Team"] = ctx.Org.Team
					break
				}
			}

			if !teamExists {
				ctx.NotFound(err)
				return
			}

			ctx.Data["IsTeamMember"] = ctx.Org.IsTeamMember
			if opts.RequireTeamMember && !ctx.Org.IsTeamMember {
				ctx.NotFound(err)
				return
			}

			ctx.Org.IsTeamAdmin = ctx.Org.Team.IsOwnerTeam() || ctx.Org.Team.AccessMode >= perm.AccessModeAdmin
			ctx.Data["IsTeamAdmin"] = ctx.Org.IsTeamAdmin
			if opts.RequireTeamAdmin && !ctx.Org.IsTeamAdmin {
				ctx.NotFound(err)
				return
			}
		}
		ctx.Data["ContextUser"] = ctx.ContextUser

		ctx.Data["CanReadProjects"] = ctx.Org.CanReadUnit(ctx, unit.TypeProjects)
		ctx.Data["CanReadPackages"] = ctx.Org.CanReadUnit(ctx, unit.TypePackages)
		ctx.Data["CanReadCode"] = ctx.Org.CanReadUnit(ctx, unit.TypeCode)

		ctx.Data["IsFollowing"] = ctx.Doer != nil && user_model.IsFollowing(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
		if len(ctx.ContextUser.Description) != 0 {
			content, err := markdown.RenderString(markup.NewRenderContext(ctx), ctx.ContextUser.Description)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			ctx.Data["RenderedDescription"] = content
		}
	}
}
