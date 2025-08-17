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

func (g *RepoGroup) CanWriteUnit(ctx *Context, unitType unit.Type) bool {
	return g.UnitPermission(ctx, ctx.Doer, unitType) >= perm.AccessModeWrite
}

func (g *RepoGroup) CanReadUnit(ctx *Context, unitType unit.Type) bool {
	return g.UnitPermission(ctx, ctx.Doer, unitType) >= perm.AccessModeRead
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

func GetGroupByParams(ctx *Context) {
	groupID := ctx.PathParamInt64("group_id")

	var err error
	ctx.RepoGroup.Group, err = group_model.GetGroupByID(ctx, groupID)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}
	if err = ctx.RepoGroup.Group.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
	}
}

type GroupAssignmentOptions struct {
	RequireMember     bool
	RequireOwner      bool
	RequireGroupAdmin bool
}

func GroupAssignment(args GroupAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		var err error

		if ctx.RepoGroup.Group == nil {
			GetGroupByParams(ctx)
			if ctx.Written() {
				return
			}
		}

		group := ctx.RepoGroup.Group
		if ctx.RepoGroup.Group.Visibility != structs.VisibleTypePublic && !ctx.IsSigned {
			ctx.NotFound(err)
			return
		}
		if ctx.RepoGroup.Group.Visibility == structs.VisibleTypePrivate {
			args.RequireMember = true
		} else if ctx.IsSigned && ctx.Doer.IsRestricted {
			args.RequireMember = true
		}
		if ctx.IsSigned && ctx.Doer.IsAdmin {
			ctx.RepoGroup.IsOwner = true
			ctx.RepoGroup.IsMember = true
			ctx.RepoGroup.IsGroupAdmin = true
			ctx.RepoGroup.CanCreateRepoOrGroup = true
		} else if ctx.IsSigned {
			ctx.RepoGroup.IsOwner, err = group.IsOwnedBy(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("IsOwnedBy", err)
				return
			}

			if ctx.RepoGroup.IsOwner {
				ctx.RepoGroup.IsMember = true
				ctx.RepoGroup.IsGroupAdmin = true
				ctx.RepoGroup.CanCreateRepoOrGroup = true
			} else {
				ctx.RepoGroup.IsMember, err = shared_group.IsGroupMember(ctx, group.ID, ctx.Doer)
				if err != nil {
					ctx.ServerError("IsOrgMember", err)
					return
				}
				ctx.RepoGroup.CanCreateRepoOrGroup, err = group.CanCreateIn(ctx, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("CanCreateIn", err)
					return
				}
			}
		} else {
			ctx.Data["SignedUser"] = &user_model.User{}
		}
		if (args.RequireMember && !ctx.RepoGroup.IsMember) ||
			(args.RequireOwner && !ctx.RepoGroup.IsOwner) {
			ctx.NotFound(err)
			return
		}
		ctx.Data["EnableFeed"] = setting.Other.EnableFeed
		ctx.Data["FeedURL"] = ctx.RepoGroup.Group.GroupLink()
		ctx.Data["IsGroupOwner"] = ctx.RepoGroup.IsOwner
		ctx.Data["IsGroupMember"] = ctx.RepoGroup.IsMember
		ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
		ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
		ctx.Data["IsPublicMember"] = func(uid int64) bool {
			is, _ := organization.IsPublicMembership(ctx, ctx.Org.Organization.ID, uid)
			return is
		}
		ctx.Data["CanReadProjects"] = ctx.RepoGroup.CanReadUnit(ctx, unit.TypeProjects)
		ctx.Data["CanCreateOrgRepo"] = ctx.RepoGroup.CanCreateRepoOrGroup

		ctx.RepoGroup.GroupLink = group.GroupLink()
		ctx.RepoGroup.OrgGroupLink = group.OrgGroupLink()

		if ctx.RepoGroup.IsMember {
			shouldSeeAllTeams := false
			if ctx.RepoGroup.IsOwner {
				shouldSeeAllTeams = true
			} else {
				teams, err := shared_group.GetGroupTeams(ctx, group.ID)
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
				ctx.RepoGroup.Teams, err = shared_group.GetGroupTeams(ctx, group.ID)
				if err != nil {
					ctx.ServerError("LoadTeams", err)
					return
				}
			} else {
				ctx.RepoGroup.Teams, err = organization.GetUserGroupTeams(ctx, group.ID, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("GetUserTeams", err)
					return
				}
			}
			ctx.Data["NumTeams"] = len(ctx.RepoGroup.Teams)
		}

		teamName := ctx.PathParam("team")
		if len(teamName) > 0 {
			teamExists := false
			for _, team := range ctx.RepoGroup.Teams {
				if strings.EqualFold(team.LowerName, strings.ToLower(teamName)) {
					teamExists = true
					var groupTeam *group_model.RepoGroupTeam
					groupTeam, err = group_model.FindGroupTeamByTeamID(ctx, group.ID, team.ID)
					if err != nil {
						ctx.ServerError("FindGroupTeamByTeamID", err)
						return
					}
					ctx.RepoGroup.GroupTeam = groupTeam
					ctx.RepoGroup.Team = team
					ctx.RepoGroup.IsMember = true
					ctx.Data["Team"] = ctx.RepoGroup.Team
					break
				}
			}

			if !teamExists {
				ctx.NotFound(err)
				return
			}

			ctx.Data["IsTeamMember"] = ctx.RepoGroup.IsMember
			if args.RequireMember && !ctx.RepoGroup.IsMember {
				ctx.NotFound(err)
				return
			}

			ctx.RepoGroup.IsGroupAdmin = ctx.RepoGroup.Team.IsOwnerTeam() || ctx.RepoGroup.Team.AccessMode >= perm.AccessModeAdmin
		} else {
			for _, team := range ctx.RepoGroup.Teams {
				if team.AccessMode >= perm.AccessModeAdmin {
					ctx.RepoGroup.IsGroupAdmin = true
					break
				}
			}
		}
		if ctx.IsSigned {
			isAdmin, err := group.IsAdminOf(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("IsAdminOf", err)
				return
			}
			ctx.RepoGroup.IsGroupAdmin = ctx.RepoGroup.IsGroupAdmin || isAdmin
		}

		ctx.Data["IsGroupAdmin"] = ctx.RepoGroup.IsGroupAdmin
		if args.RequireGroupAdmin && !ctx.RepoGroup.IsGroupAdmin {
			ctx.NotFound(err)
			return
		}

		if len(ctx.RepoGroup.Group.Description) != 0 {
			ctx.Data["RenderedDescription"], err = markdown.RenderString(markup.NewRenderContext(ctx), ctx.RepoGroup.Group.Description)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
		}
		ctx.Data["Group"] = ctx.RepoGroup.Group
		ctx.Data["ContextGroup"] = ctx.RepoGroup
		ctx.Data["Doer"] = ctx.Doer
		ctx.Data["GroupLink"] = ctx.RepoGroup.Group.GroupLink()
		ctx.Data["OrgGroupLink"] = ctx.RepoGroup.OrgGroupLink
		ctx.Data["Breadcrumbs"], err = group_model.GetParentGroupChain(ctx, group.ID)
		if err != nil {
			ctx.ServerError("GetParentGroupChain", err)
			return
		}
	}
}

func groupIsCurrent(ctx *Context) func(groupID int64) bool {
	return func(groupID int64) bool {
		if ctx.RepoGroup.Group == nil {
			return false
		}
		return ctx.RepoGroup.Group.ID == groupID
	}
}
