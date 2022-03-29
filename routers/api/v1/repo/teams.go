// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

// ListTeams list a repository's teams
func ListTeams(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/teams repository repoListTeams
	// ---
	// summary: List a repository's teams
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"

	if !ctx.Repo.Owner.IsOrganization() {
		ctx.Error(http.StatusMethodNotAllowed, "noOrg", "repo is not owned by an organization")
		return
	}

	teams, err := organization.GetRepoTeams(ctx.Repo.Repository)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiTeams := make([]*api.Team, len(teams))
	for i := range teams {
		if err := teams[i].GetUnits(); err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUnits", err)
			return
		}

		apiTeams[i] = convert.ToTeam(teams[i])
	}

	ctx.JSON(http.StatusOK, apiTeams)
}

// IsTeam check if a team is assigned to a repository
func IsTeam(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/teams/{team} repository repoCheckTeam
	// ---
	// summary: Check if a team is assigned to a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
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
	//   "405":
	//     "$ref": "#/responses/error"

	if !ctx.Repo.Owner.IsOrganization() {
		ctx.Error(http.StatusMethodNotAllowed, "noOrg", "repo is not owned by an organization")
		return
	}

	team := getTeamByParam(ctx)
	if team == nil {
		return
	}

	if models.HasRepository(team, ctx.Repo.Repository.ID) {
		if err := team.GetUnits(); err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUnits", err)
			return
		}
		apiTeam := convert.ToTeam(team)
		ctx.JSON(http.StatusOK, apiTeam)
		return
	}

	ctx.NotFound()
}

// AddTeam add a team to a repository
func AddTeam(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/teams/{team} repository repoAddTeam
	// ---
	// summary: Add a team to a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
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
	//   "405":
	//     "$ref": "#/responses/error"

	changeRepoTeam(ctx, true)
}

// DeleteTeam delete a team from a repository
func DeleteTeam(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/teams/{team} repository repoDeleteTeam
	// ---
	// summary: Delete a team from a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
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
	//   "405":
	//     "$ref": "#/responses/error"

	changeRepoTeam(ctx, false)
}

func changeRepoTeam(ctx *context.APIContext, add bool) {
	if !ctx.Repo.Owner.IsOrganization() {
		ctx.Error(http.StatusMethodNotAllowed, "noOrg", "repo is not owned by an organization")
	}
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Error(http.StatusForbidden, "noAdmin", "user is nor repo admin nor owner")
		return
	}

	team := getTeamByParam(ctx)
	if team == nil {
		return
	}

	repoHasTeam := models.HasRepository(team, ctx.Repo.Repository.ID)
	var err error
	if add {
		if repoHasTeam {
			ctx.Error(http.StatusUnprocessableEntity, "alreadyAdded", fmt.Errorf("team '%s' is already added to repo", team.Name))
			return
		}
		err = models.AddRepository(team, ctx.Repo.Repository)
	} else {
		if !repoHasTeam {
			ctx.Error(http.StatusUnprocessableEntity, "notAdded", fmt.Errorf("team '%s' was not added to repo", team.Name))
			return
		}
		err = models.RemoveRepository(team, ctx.Repo.Repository.ID)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func getTeamByParam(ctx *context.APIContext) *organization.Team {
	team, err := organization.GetTeam(ctx.Repo.Owner.ID, ctx.Params(":team"))
	if err != nil {
		if organization.IsErrTeamNotExist(err) {
			ctx.Error(http.StatusNotFound, "TeamNotExit", err)
			return nil
		}
		ctx.InternalServerError(err)
		return nil
	}
	return team
}
