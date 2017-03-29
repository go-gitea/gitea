// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/user"
)

// ListTeams list all the teams of an organization
func ListTeams(ctx *context.APIContext) {
	org := ctx.Org.Organization
	if err := org.GetTeams(); err != nil {
		ctx.Error(500, "GetTeams", err)
		return
	}

	apiTeams := make([]*api.Team, len(org.Teams))
	for i := range org.Teams {
		apiTeams[i] = convert.ToTeam(org.Teams[i])
	}
	ctx.JSON(200, apiTeams)
}

// GetTeam api for get a team
func GetTeam(ctx *context.APIContext) {
	ctx.JSON(200, convert.ToTeam(ctx.Org.Team))
}

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

// GetTeamMembers api for get a team's members
func GetTeamMembers(ctx *context.APIContext) {
	if !models.IsOrganizationMember(ctx.Org.Team.OrgID, ctx.User.ID) {
		ctx.Status(404)
		return
	}
	team := ctx.Org.Team
	if err := team.GetMembers(); err != nil {
		ctx.Error(500, "GetTeamMembers", err)
		return
	}
	members := make([]*api.User, len(team.Members))
	for i, member := range team.Members {
		members[i] = member.APIFormat()
	}
	ctx.JSON(200, members)
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

// GetTeamRepos api for get a team's repos
func GetTeamRepos(ctx *context.APIContext) {
	team := ctx.Org.Team
	if err := team.GetRepositories(); err != nil {
		ctx.Error(500, "GetTeamRepos", err)
	}
	repos := make([]*api.Repository, len(team.Repos))
	for i, repo := range team.Repos {
		access, err := models.AccessLevel(ctx.User.ID, repo)
		if err != nil {
			ctx.Error(500, "GetTeamRepos", err)
			return
		}
		repos[i] = repo.APIFormat(access)
	}
	ctx.JSON(200, repos)
}

// getRepositoryByParams get repository by a team's organization ID and repo name
func getRepositoryByParams(ctx *context.APIContext) *models.Repository {
	repo, err := models.GetRepositoryByName(ctx.Org.Team.OrgID, ctx.Params(":reponame"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetRepositoryByName", err)
		}
		return nil
	}
	return repo
}

// AddTeamRepository api for adding a repository to a team
func AddTeamRepository(ctx *context.APIContext) {
	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User.ID, repo); err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(403, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.AddRepository(repo); err != nil {
		ctx.Error(500, "AddRepository", err)
		return
	}
	ctx.Status(204)
}

// RemoveTeamRepository api for removing a repository from a team
func RemoveTeamRepository(ctx *context.APIContext) {
	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User.ID, repo); err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(403, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.RemoveRepository(repo.ID); err != nil {
		ctx.Error(500, "RemoveRepository", err)
		return
	}
	ctx.Status(204)
}
