// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/convert"
	org_service "code.gitea.io/gitea/services/org"
	repo_service "code.gitea.io/gitea/services/repository"
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.Owner.IsOrganization() {
		ctx.Error(http.StatusMethodNotAllowed, "noOrg", "repo is not owned by an organization")
		return
	}

	teams, err := organization.GetRepoTeams(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiTeams, err := convert.ToTeams(ctx, teams, false)
	if err != nil {
		ctx.InternalServerError(err)
		return
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

	if repo_service.HasRepository(ctx, team, ctx.Repo.Repository.ID) {
		apiTeam, err := convert.ToTeam(ctx, team)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
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
	//   "404":
	//     "$ref": "#/responses/notFound"

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
	//   "404":
	//     "$ref": "#/responses/notFound"

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

	repoHasTeam := repo_service.HasRepository(ctx, team, ctx.Repo.Repository.ID)
	var err error
	if add {
		if repoHasTeam {
			ctx.Error(http.StatusUnprocessableEntity, "alreadyAdded", fmt.Errorf("team '%s' is already added to repo", team.Name))
			return
		}
		err = org_service.TeamAddRepository(ctx, team, ctx.Repo.Repository)
	} else {
		if !repoHasTeam {
			ctx.Error(http.StatusUnprocessableEntity, "notAdded", fmt.Errorf("team '%s' was not added to repo", team.Name))
			return
		}
		err = repo_service.RemoveRepositoryFromTeam(ctx, team, ctx.Repo.Repository.ID)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func getTeamByParam(ctx *context.APIContext) *organization.Team {
	team, err := organization.GetTeam(ctx, ctx.Repo.Owner.ID, ctx.Params(":team"))
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
