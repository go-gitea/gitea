// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListTeams list all the teams of an organization
func ListTeams(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/teams organization orgListTeams
	// ---
	// summary: List an organization's teams
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"

	teams, count, err := organization.SearchTeam(&organization.SearchTeamOptions{
		ListOptions: utils.GetListOptions(ctx),
		OrgID:       ctx.Org.Organization.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadTeams", err)
		return
	}

	apiTeams, err := convert.ToTeams(teams, false)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ConvertToTeams", err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiTeams)
}

// ListUserTeams list all the teams a user belongs to
func ListUserTeams(ctx *context.APIContext) {
	// swagger:operation GET /user/teams user userListTeams
	// ---
	// summary: List all the teams a user belongs to
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"

	teams, count, err := organization.SearchTeam(&organization.SearchTeamOptions{
		ListOptions: utils.GetListOptions(ctx),
		UserID:      ctx.Doer.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserTeams", err)
		return
	}

	apiTeams, err := convert.ToTeams(teams, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ConvertToTeams", err)
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiTeams)
}

// GetTeam api for get a team
func GetTeam(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id} organization orgGetTeam
	// ---
	// summary: Get a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"

	apiTeam, err := convert.ToTeam(ctx.Org.Team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, apiTeam)
}

func attachTeamUnits(team *organization.Team, units []string) {
	unitTypes := unit_model.FindUnitTypes(units...)
	team.Units = make([]*organization.TeamUnit, 0, len(units))
	for _, tp := range unitTypes {
		team.Units = append(team.Units, &organization.TeamUnit{
			OrgID:      team.OrgID,
			Type:       tp,
			AccessMode: team.AccessMode,
		})
	}
}

func convertUnitsMap(unitsMap map[string]string) map[unit_model.Type]perm.AccessMode {
	res := make(map[unit_model.Type]perm.AccessMode, len(unitsMap))
	for unitKey, p := range unitsMap {
		res[unit_model.TypeFromKey(unitKey)] = perm.ParseAccessMode(p)
	}
	return res
}

func attachTeamUnitsMap(team *organization.Team, unitsMap map[string]string) {
	team.Units = make([]*organization.TeamUnit, 0, len(unitsMap))
	for unitKey, p := range unitsMap {
		team.Units = append(team.Units, &organization.TeamUnit{
			OrgID:      team.OrgID,
			Type:       unit_model.TypeFromKey(unitKey),
			AccessMode: perm.ParseAccessMode(p),
		})
	}
}

// CreateTeam api for create a team
func CreateTeam(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/teams organization orgCreateTeam
	// ---
	// summary: Create a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateTeamOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Team"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.CreateTeamOption)
	p := perm.ParseAccessMode(form.Permission)
	if p < perm.AccessModeAdmin && len(form.UnitsMap) > 0 {
		p = unit_model.MinUnitAccessMode(convertUnitsMap(form.UnitsMap))
	}
	team := &organization.Team{
		OrgID:                   ctx.Org.Organization.ID,
		Name:                    form.Name,
		Description:             form.Description,
		IncludesAllRepositories: form.IncludesAllRepositories,
		CanCreateOrgRepo:        form.CanCreateOrgRepo,
		AccessMode:              p,
	}

	if team.AccessMode < perm.AccessModeAdmin {
		if len(form.UnitsMap) > 0 {
			attachTeamUnitsMap(team, form.UnitsMap)
		} else if len(form.Units) > 0 {
			attachTeamUnits(team, form.Units)
		} else {
			ctx.Error(http.StatusInternalServerError, "getTeamUnits", errors.New("units permission should not be empty"))
			return
		}
	}

	if err := models.NewTeam(team); err != nil {
		if organization.IsErrTeamAlreadyExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "NewTeam", err)
		}
		return
	}

	apiTeam, err := convert.ToTeam(team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, apiTeam)
}

// EditTeam api for edit a team
func EditTeam(ctx *context.APIContext) {
	// swagger:operation PATCH /teams/{id} organization orgEditTeam
	// ---
	// summary: Edit a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to edit
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditTeamOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"

	form := web.GetForm(ctx).(*api.EditTeamOption)
	team := ctx.Org.Team
	if err := team.GetUnits(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	if form.CanCreateOrgRepo != nil {
		team.CanCreateOrgRepo = team.IsOwnerTeam() || *form.CanCreateOrgRepo
	}

	if len(form.Name) > 0 {
		team.Name = form.Name
	}

	if form.Description != nil {
		team.Description = *form.Description
	}

	isAuthChanged := false
	isIncludeAllChanged := false
	if !team.IsOwnerTeam() && len(form.Permission) != 0 {
		// Validate permission level.
		p := perm.ParseAccessMode(form.Permission)
		if p < perm.AccessModeAdmin && len(form.UnitsMap) > 0 {
			p = unit_model.MinUnitAccessMode(convertUnitsMap(form.UnitsMap))
		}

		if team.AccessMode != p {
			isAuthChanged = true
			team.AccessMode = p
		}

		if form.IncludesAllRepositories != nil {
			isIncludeAllChanged = true
			team.IncludesAllRepositories = *form.IncludesAllRepositories
		}
	}

	if team.AccessMode < perm.AccessModeAdmin {
		if len(form.UnitsMap) > 0 {
			attachTeamUnitsMap(team, form.UnitsMap)
		} else if len(form.Units) > 0 {
			attachTeamUnits(team, form.Units)
		}
	}

	if err := models.UpdateTeam(team, isAuthChanged, isIncludeAllChanged); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditTeam", err)
		return
	}

	apiTeam, err := convert.ToTeam(team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, apiTeam)
}

// DeleteTeam api for delete a team
func DeleteTeam(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id} organization orgDeleteTeam
	// ---
	// summary: Delete a team
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: team deleted

	if err := models.DeleteTeam(ctx.Org.Team); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteTeam", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetTeamMembers api for get a team's members
func GetTeamMembers(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members organization orgListTeamMembers
	// ---
	// summary: List a team's members
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	isMember, err := organization.IsOrganizationMember(ctx, ctx.Org.Team.OrgID, ctx.Doer.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
		return
	} else if !isMember && !ctx.Doer.IsAdmin {
		ctx.NotFound()
		return
	}

	teamMembers, err := organization.GetTeamMembers(ctx, &organization.SearchMembersOptions{
		ListOptions: utils.GetListOptions(ctx),
		TeamID:      ctx.Org.Team.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeamMembers", err)
		return
	}

	members := make([]*api.User, len(teamMembers))
	for i, member := range teamMembers {
		members[i] = convert.ToUser(member, ctx.Doer)
	}

	ctx.SetTotalCountHeader(int64(ctx.Org.Team.NumMembers))
	ctx.JSON(http.StatusOK, members)
}

// GetTeamMember api for get a particular member of team
func GetTeamMember(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members/{username} organization orgListTeamMember
	// ---
	// summary: List a particular member of team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the member to list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	teamID := ctx.ParamsInt64("teamid")
	isTeamMember, err := organization.IsUserInTeams(ctx, u.ID, []int64{teamID})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsUserInTeams", err)
		return
	} else if !isTeamMember {
		ctx.NotFound()
		return
	}
	ctx.JSON(http.StatusOK, convert.ToUser(u, ctx.Doer))
}

// AddTeamMember api for add a member to a team
func AddTeamMember(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/members/{username} organization orgAddTeamMember
	// ---
	// summary: Add a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.AddTeamMember(ctx.Org.Team, u.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddMember", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// RemoveTeamMember api for remove one member from a team
func RemoveTeamMember(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/members/{username} organization orgRemoveTeamMember
	// ---
	// summary: Remove a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	if err := models.RemoveTeamMember(ctx.Org.Team, u.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveTeamMember", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetTeamRepos api for get a team's repos
func GetTeamRepos(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/repos organization orgListTeamRepos
	// ---
	// summary: List a team's repos
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"

	team := ctx.Org.Team
	teamRepos, err := organization.GetTeamRepositories(ctx, &organization.SearchTeamRepoOptions{
		ListOptions: utils.GetListOptions(ctx),
		TeamID:      team.ID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeamRepos", err)
		return
	}
	repos := make([]*api.Repository, len(teamRepos))
	for i, repo := range teamRepos {
		access, err := access_model.AccessLevel(ctx.Doer, repo)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetTeamRepos", err)
			return
		}
		repos[i] = convert.ToRepo(repo, access)
	}
	ctx.SetTotalCountHeader(int64(team.NumRepos))
	ctx.JSON(http.StatusOK, repos)
}

// GetTeamRepo api for get a particular repo of team
func GetTeamRepo(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/repos/{org}/{repo} organization orgListTeamRepo
	// ---
	// summary: List a particular repo of team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to list
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}

	if !organization.HasTeamRepo(ctx, ctx.Org.Team.OrgID, ctx.Org.Team.ID, repo.ID) {
		ctx.NotFound()
		return
	}

	access, err := access_model.AccessLevel(ctx.Doer, repo)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeamRepos", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(repo, access))
}

// getRepositoryByParams get repository by a team's organization ID and repo name
func getRepositoryByParams(ctx *context.APIContext) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByName(ctx.Org.Team.OrgID, ctx.Params(":reponame"))
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
		}
		return nil
	}
	return repo
}

// AddTeamRepository api for adding a repository to a team
func AddTeamRepository(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/repos/{org}/{repo} organization orgAddTeamRepository
	// ---
	// summary: Add a repository to a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to add
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := access_model.AccessLevel(ctx.Doer, repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AccessLevel", err)
		return
	} else if access < perm.AccessModeAdmin {
		ctx.Error(http.StatusForbidden, "", "Must have admin-level access to the repository")
		return
	}
	if err := models.AddRepository(ctx.Org.Team, repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddRepository", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// RemoveTeamRepository api for removing a repository from a team
func RemoveTeamRepository(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/repos/{org}/{repo} organization orgRemoveTeamRepository
	// ---
	// summary: Remove a repository from a team
	// description: This does not delete the repository, it only removes the
	//              repository from the team.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to remove
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := access_model.AccessLevel(ctx.Doer, repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AccessLevel", err)
		return
	} else if access < perm.AccessModeAdmin {
		ctx.Error(http.StatusForbidden, "", "Must have admin-level access to the repository")
		return
	}
	if err := models.RemoveRepository(ctx.Org.Team, repo.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveRepository", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// SearchTeam api for searching teams
func SearchTeam(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/teams/search organization teamSearch
	// ---
	// summary: Search for teams within an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keywords to search
	//   type: string
	// - name: include_desc
	//   in: query
	//   description: include search within team description (defaults to true)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     description: "SearchResults of a successful search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/definitions/Team"

	listOptions := utils.GetListOptions(ctx)

	opts := &organization.SearchTeamOptions{
		UserID:      ctx.Doer.ID,
		Keyword:     ctx.FormTrim("q"),
		OrgID:       ctx.Org.Organization.ID,
		IncludeDesc: ctx.FormString("include_desc") == "" || ctx.FormBool("include_desc"),
		ListOptions: listOptions,
	}

	teams, maxResults, err := organization.SearchTeam(opts)
	if err != nil {
		log.Error("SearchTeam failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "SearchTeam internal failure",
		})
		return
	}

	apiTeams, err := convert.ToTeams(teams, false)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": apiTeams,
	})
}
