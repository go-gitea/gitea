// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	org_model "code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	org_group_model "code.gitea.io/gitea/models/shared/group"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	group_service "code.gitea.io/gitea/services/group"
)

// ListTeams list a repository group's teams
func ListTeams(ctx *context.APIContext) {
	// swagger:operation GET /groups/{group_id}/teams repository-group repoGroupListTeams
	// ---
	// summary: List a repository group's teams
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	var (
		err   error
		group *group_model.Group
	)
	group, err = group_model.GetGroupByID(ctx, ctx.PathParamInt64("group_id"))
	if group_model.IsErrGroupNotExist(err) {
		ctx.APIErrorNotFound()
		return
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	teams, err := org_group_model.GetGroupTeams(ctx, group.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiTeams, err := convert.ToTeams(ctx, teams, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiTeams)
}

// AddTeam add a team to a repository group
func AddTeam(ctx *context.APIContext) {
	// swagger:operation PUT /groups/{group_id}/teams/{team} repository-group repoGroupAddTeam
	// ---
	// summary: Add a team to a repository group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group
	//   type: integer
	//   format: int64
	//   required: true
	// - name: team
	//   in: path
	//   description: team name
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateRepoGroupTeamOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.CreateOrUpdateRepoGroupTeamOption)

	changeGroupTeam(ctx, form, true)
}

// EditTeam update a team assigned to a repository group
func EditTeam(ctx *context.APIContext) {
	// swagger:operation PATCH /groups/{group_id}/teams/{team} repository-group repoGroupEditTeam
	// ---
	// summary: Update a team assigned to a repository group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group
	//   type: integer
	//   format: int64
	//   required: true
	// - name: team
	//   in: path
	//   description: team name
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateRepoGroupTeamOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.CreateOrUpdateRepoGroupTeamOption)

	gid := ctx.PathParamInt64("group_id")
	group, err := group_model.GetGroupByID(ctx, gid)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			ctx.APIErrorNotFound()
		}
		ctx.APIErrorInternal(err)
		return
	}
	team := getTeamFromGroup(ctx, group)
	gt, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, team.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if gt == nil {
		ctx.APIErrorNotFound()
		return
	}
	if form.CanCreateIn != nil {
		gt.CanCreateIn = *form.CanCreateIn
	}
	if form.Permission != nil {
		gt.AccessMode = perm_model.ParseAccessMode(string(*form.Permission))
	}
	err = group_service.UpdateGroupTeam(ctx, gt, form.UnitsMap)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteTeam delete a team from a repository group
func DeleteTeam(ctx *context.APIContext) {
	// swagger:operation DELETE /groups/{group_id}/teams/{team} repository-group repoGroupDeleteTeam
	// ---
	// summary: Add a team to a repository group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group
	//   type: integer
	//   format: int64
	//   required: true
	// - name: team
	//   in: path
	//   description: team name
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	changeGroupTeam(ctx, nil, false)
}

// IsTeam check if a team is assigned to a repository
func IsTeam(ctx *context.APIContext) {
	// swagger:operation GET /groups/{group_id}/teams/{team} repository-group repoGroupCheckTeam
	// ---
	// summary: Check if a team is assigned to a repository group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group
	//   type: integer
	//   format: int64
	//   required: true
	// - name: team
	//   in: path
	//   description: team name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"
	//   "404":
	//     "$ref": "#/responses/notFound"

	gid := ctx.PathParamInt64("group_id")
	group, err := group_model.GetGroupByID(ctx, gid)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	team := getTeamFromGroup(ctx, group)
	if team == nil {
		return
	}

	if group_model.HasTeamGroup(ctx, group.OwnerID, team.ID, gid) {
		apiTeam, err := convert.ToTeam(ctx, team)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.JSON(http.StatusOK, apiTeam)
		return
	}

	ctx.APIErrorNotFound()
}

func getTeamFromGroup(ctx *context.APIContext, group *group_model.Group) *org_model.Team {
	teamName := ctx.PathParam("team")

	team, err := org_model.GetTeam(ctx, group.OwnerID, teamName)
	if err != nil {
		if org_model.IsErrTeamNotExist(err) {
			ctx.APIErrorNotFound()
			return nil
		}
		ctx.APIErrorInternal(err)
		return nil
	}
	return team
}

func changeGroupTeam(ctx *context.APIContext, options *api.CreateOrUpdateRepoGroupTeamOption, add bool) {
	gid := ctx.PathParamInt64("group_id")
	group, err := group_model.GetGroupByID(ctx, gid)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	err = group.LoadOwner(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	team := getTeamFromGroup(ctx, group)
	if team == nil {
		return
	}

	groupHasTeam := group_model.HasTeamGroup(ctx, group.OwnerID, team.ID, gid)

	if add {
		if groupHasTeam {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("team '%s' is already added to group", team.Name))
			return
		}
		var accessModeArg *perm_model.AccessMode
		if options.Permission != nil {
			accessModeArg = new(perm_model.ParseAccessMode(string(*options.Permission)))
		}
		err = group_service.AddTeamToGroup(ctx, group, team.Name, options.UnitsMap, options.CanCreateIn, accessModeArg)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	} else {
		err = group_service.DeleteTeamFromGroup(ctx, group, group.OwnerID, team.Name)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if _, err = db.GetEngine(ctx).Where("group_id = ?", gid).Delete(new(group_model.RepoGroupUnit)); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	ctx.Status(http.StatusNoContent)
}
