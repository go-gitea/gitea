// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
)

// Organization contains organization context
type Organization struct {
	IsOwner          bool
	IsMember         bool
	IsTeamMember     bool // Is member of team.
	IsTeamAdmin      bool // In owner team or team that has admin permission level.
	Organization     *models.Organization
	OrgLink          string
	CanCreateOrgRepo bool

	Team  *models.Team
	Teams []*models.Team
}

// HandleOrgAssignment handles organization assignment
func HandleOrgAssignment(ctx *Context, args ...bool) {
	var (
		requireMember     bool
		requireOwner      bool
		requireTeamMember bool
		requireTeamAdmin  bool
	)
	if len(args) >= 1 {
		requireMember = args[0]
	}
	if len(args) >= 2 {
		requireOwner = args[1]
	}
	if len(args) >= 3 {
		requireTeamMember = args[2]
	}
	if len(args) >= 4 {
		requireTeamAdmin = args[3]
	}

	orgName := ctx.Params(":org")

	var err error
	ctx.Org.Organization, err = models.GetOrgByName(orgName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			redirectUserID, err := user_model.LookupUserRedirect(orgName)
			if err == nil {
				RedirectToUser(ctx, orgName, redirectUserID)
			} else if user_model.IsErrUserRedirectNotExist(err) {
				ctx.NotFound("GetUserByName", err)
			} else {
				ctx.ServerError("LookupUserRedirect", err)
			}
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}
	org := ctx.Org.Organization
	ctx.Data["Org"] = org

	teams, err := org.LoadTeams()
	if err != nil {
		ctx.ServerError("LoadTeams", err)
	}
	ctx.Data["OrgTeams"] = teams

	// Admin has super access.
	if ctx.IsSigned && ctx.User.IsAdmin {
		ctx.Org.IsOwner = true
		ctx.Org.IsMember = true
		ctx.Org.IsTeamMember = true
		ctx.Org.IsTeamAdmin = true
		ctx.Org.CanCreateOrgRepo = true
	} else if ctx.IsSigned {
		ctx.Org.IsOwner, err = org.IsOwnedBy(ctx.User.ID)
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
			ctx.Org.IsMember, err = org.IsOrgMember(ctx.User.ID)
			if err != nil {
				ctx.ServerError("IsOrgMember", err)
				return
			}
			ctx.Org.CanCreateOrgRepo, err = org.CanCreateOrgRepo(ctx.User.ID)
			if err != nil {
				ctx.ServerError("CanCreateOrgRepo", err)
				return
			}
		}
	} else {
		// Fake data.
		ctx.Data["SignedUser"] = &user_model.User{}
	}
	if (requireMember && !ctx.Org.IsMember) ||
		(requireOwner && !ctx.Org.IsOwner) {
		ctx.NotFound("OrgAssignment", err)
		return
	}
	ctx.Data["IsOrganizationOwner"] = ctx.Org.IsOwner
	ctx.Data["IsOrganizationMember"] = ctx.Org.IsMember
	ctx.Data["IsPublicMember"] = func(uid int64) bool {
		is, _ := models.IsPublicMembership(ctx.Org.Organization.ID, uid)
		return is
	}
	ctx.Data["CanCreateOrgRepo"] = ctx.Org.CanCreateOrgRepo

	ctx.Org.OrgLink = org.AsUser().OrganisationLink()
	ctx.Data["OrgLink"] = ctx.Org.OrgLink

	// Team.
	if ctx.Org.IsMember {
		shouldSeeAllTeams := false
		if ctx.Org.IsOwner {
			shouldSeeAllTeams = true
		} else {
			teams, err := org.GetUserTeams(ctx.User.ID)
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
			ctx.Org.Teams, err = org.LoadTeams()
			if err != nil {
				ctx.ServerError("LoadTeams", err)
				return
			}
		} else {
			ctx.Org.Teams, err = org.GetUserTeams(ctx.User.ID)
			if err != nil {
				ctx.ServerError("GetUserTeams", err)
				return
			}
		}
	}

	teamName := ctx.Params(":team")
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
			ctx.NotFound("OrgAssignment", err)
			return
		}

		ctx.Data["IsTeamMember"] = ctx.Org.IsTeamMember
		if requireTeamMember && !ctx.Org.IsTeamMember {
			ctx.NotFound("OrgAssignment", err)
			return
		}

		ctx.Org.IsTeamAdmin = ctx.Org.Team.IsOwnerTeam() || ctx.Org.Team.AccessMode >= perm.AccessModeAdmin
		ctx.Data["IsTeamAdmin"] = ctx.Org.IsTeamAdmin
		if requireTeamAdmin && !ctx.Org.IsTeamAdmin {
			ctx.NotFound("OrgAssignment", err)
			return
		}
	}
}

// OrgAssignment returns a middleware to handle organization assignment
func OrgAssignment(args ...bool) func(ctx *Context) {
	return func(ctx *Context) {
		HandleOrgAssignment(ctx, args...)
	}
}
