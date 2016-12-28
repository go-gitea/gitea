// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/user"
)

// CreateTeam api for create a team
func CreateTeam(ctx *context.APIContext, form api.CreateTeamOption) {
	team := &models.Team{
		OrgID:       ctx.Org.Organization.ID,
		Name:        form.Name,
		Description: form.Description,
		Authorize:   models.ParseAccessMode(form.Permission),
	}
	if err := models.NewTeam(team); err != nil {
		if models.IsErrTeamAlreadyExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "NewTeam", err)
		}
		return
	}

	ctx.JSON(201, convert.ToTeam(team))
}

// EditTeam api for edit a team
func EditTeam(ctx *context.APIContext, form api.EditTeamOption) {
	team := &models.Team{
		ID:          ctx.Org.Team.ID,
		OrgID:       ctx.Org.Team.OrgID,
		Name:        form.Name,
		Description: form.Description,
		Authorize:   models.ParseAccessMode(form.Permission),
	}
	if err := models.UpdateTeam(team, true); err != nil {
		ctx.Error(500, "EditTeam", err)
		return
	}
	ctx.JSON(200, convert.ToTeam(team))
}

// DeleteTeam api for delete a team
func DeleteTeam(ctx *context.APIContext) {
	if err := models.DeleteTeam(ctx.Org.Team); err != nil {
		ctx.Error(500, "DeleteTeam", err)
		return
	}
	ctx.Status(204)
}

// AddTeamMember api for add a member to a team
func AddTeamMember(ctx *context.APIContext) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := ctx.Org.Team.AddMember(u.ID); err != nil {
		ctx.Error(500, "AddMember", err)
		return
	}

	ctx.Status(204)
}

// RemoveTeamMember api for remove one member from a team
func RemoveTeamMember(ctx *context.APIContext) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	if err := ctx.Org.Team.RemoveMember(u.ID); err != nil {
		ctx.Error(500, "RemoveMember", err)
		return
	}

	ctx.Status(204)
}
