// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	macaron "gopkg.in/macaron.v1"
)

// Organization contains organization context
type Organization struct {
	IsOwner      bool
	IsMember     bool
	IsTeamMember bool // Is member of team.
	IsTeamAdmin  bool // In owner team or team that has admin permission level.
	Organization *models.User
	OrgLink      string

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
			ctx.Handle(404, "GetUserByName", err)
		} else {
			ctx.Handle(500, "GetUserByName", err)
		}
		return
	}
	org := ctx.Org.Organization
	ctx.Data["Org"] = org

	// Force redirection when username is actually a user.
	if !org.IsOrganization() {
		ctx.Redirect("/" + org.Name)
		return
	}

	// Admin has super access.
	if ctx.IsSigned && ctx.User.IsAdmin {
		ctx.Org.IsOwner = true
		ctx.Org.IsMember = true
		ctx.Org.IsTeamMember = true
		ctx.Org.IsTeamAdmin = true
	} else if ctx.IsSigned {
		ctx.Org.IsOwner = org.IsOwnedBy(ctx.User.ID)
		if ctx.Org.IsOwner {
			ctx.Org.IsMember = true
			ctx.Org.IsTeamMember = true
			ctx.Org.IsTeamAdmin = true
		} else {
			if org.IsOrgMember(ctx.User.ID) {
				ctx.Org.IsMember = true
			}
		}
	} else {
		// Fake data.
		ctx.Data["SignedUser"] = &models.User{}
	}
	if (requireMember && !ctx.Org.IsMember) ||
		(requireOwner && !ctx.Org.IsOwner) {
		ctx.Handle(404, "OrgAssignment", err)
		return
	}
	ctx.Data["IsOrganizationOwner"] = ctx.Org.IsOwner
	ctx.Data["IsOrganizationMember"] = ctx.Org.IsMember

	ctx.Org.OrgLink = setting.AppSubURL + "/org/" + org.Name
	ctx.Data["OrgLink"] = ctx.Org.OrgLink

	// Team.
	if ctx.Org.IsMember {
		if ctx.Org.IsOwner {
			if err := org.GetTeams(); err != nil {
				ctx.Handle(500, "GetTeams", err)
				return
			}
		} else {
			org.Teams, err = org.GetUserTeams(ctx.User.ID)
			if err != nil {
				ctx.Handle(500, "GetUserTeams", err)
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
			ctx.Handle(404, "OrgAssignment", err)
			return
		}

		ctx.Data["IsTeamMember"] = ctx.Org.IsTeamMember
		if requireTeamMember && !ctx.Org.IsTeamMember {
			ctx.Handle(404, "OrgAssignment", err)
			return
		}

		ctx.Org.IsTeamAdmin = ctx.Org.Team.IsOwnerTeam() || ctx.Org.Team.Authorize >= models.AccessModeAdmin
		ctx.Data["IsTeamAdmin"] = ctx.Org.IsTeamAdmin
		if requireTeamAdmin && !ctx.Org.IsTeamAdmin {
			ctx.Handle(404, "OrgAssignment", err)
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
