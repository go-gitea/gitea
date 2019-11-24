// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
)

// Organization contains organization context
type Organization struct {
	IsOwner          bool
	IsMember         bool
	IsTeamMember     bool // Is member of team.
	IsTeamAdmin      bool // In owner team or team that has admin permission level.
	Organization     *models.User
	OrgLink          string
	CanCreateOrgRepo bool

	Team *models.Team
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
	ctx.Org.Organization, err = models.GetUserByName(orgName)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound("GetUserByName", err)
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}
	org := ctx.Org.Organization
	ctx.Data["Org"] = org

	// Force redirection when username is actually a user.
	if !org.IsOrganization() {
		ctx.Redirect(setting.AppSubURL + "/" + org.Name)
		return
	}

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
		ctx.Data["SignedUser"] = &models.User{}
	}
	if (requireMember && !ctx.Org.IsMember) ||
		(requireOwner && !ctx.Org.IsOwner) {
		ctx.NotFound("OrgAssignment", err)
		return
	}
	ctx.Data["IsOrganizationOwner"] = ctx.Org.IsOwner
	ctx.Data["IsOrganizationMember"] = ctx.Org.IsMember
	ctx.Data["CanCreateOrgRepo"] = ctx.Org.CanCreateOrgRepo

	ctx.Org.OrgLink = setting.AppSubURL + "/org/" + org.Name
	ctx.Data["OrgLink"] = ctx.Org.OrgLink

	// Team.
	if ctx.Org.IsMember {
		if ctx.Org.IsOwner {
			if err := org.GetTeams(); err != nil {
				ctx.ServerError("GetTeams", err)
				return
			}
		} else {
			org.Teams, err = org.GetUserTeams(ctx.User.ID)
			if err != nil {
				ctx.ServerError("GetUserTeams", err)
				return
			}
		}
	}

	teamName := ctx.Params(":team")
	if len(teamName) > 0 {
		teamExists := false
		for _, team := range org.Teams {
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

		ctx.Org.IsTeamAdmin = ctx.Org.Team.IsOwnerTeam() || ctx.Org.Team.Authorize >= models.AccessModeAdmin
		ctx.Data["IsTeamAdmin"] = ctx.Org.IsTeamAdmin
		if requireTeamAdmin && !ctx.Org.IsTeamAdmin {
			ctx.NotFound("OrgAssignment", err)
			return
		}
	}
}

// OrgAssignment returns a macaron middleware to handle organization assignment
func OrgAssignment(args ...bool) macaron.Handler {
	return func(ctx *Context) {
		HandleOrgAssignment(ctx, args...)
	}
}
