// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/forms"
)

const (
	// tplTeams template path for teams list page
	tplTeams base.TplName = "org/team/teams"
	// tplTeamNew template path for create new team page
	tplTeamNew base.TplName = "org/team/new"
	// tplTeamMembers template path for showing team members page
	tplTeamMembers base.TplName = "org/team/members"
	// tplTeamRepositories template path for showing team repositories page
	tplTeamRepositories base.TplName = "org/team/repositories"
)

// Teams render teams list page
func Teams(ctx *context.Context) {
	org := ctx.Org.Organization
	ctx.Data["Title"] = org.FullName
	ctx.Data["PageIsOrgTeams"] = true

	for _, t := range ctx.Org.Teams {
		if err := t.GetMembersCtx(ctx); err != nil {
			ctx.ServerError("GetMembers", err)
			return
		}
	}
	ctx.Data["Teams"] = ctx.Org.Teams

	ctx.HTML(http.StatusOK, tplTeams)
}

// TeamsAction response for join, leave, remove, add operations to team
func TeamsAction(ctx *context.Context) {
	uid := ctx.FormInt64("uid")
	if uid == 0 {
		ctx.Redirect(ctx.Org.OrgLink + "/teams")
		return
	}

	page := ctx.FormString("page")
	var err error
	switch ctx.Params(":action") {
	case "join":
		if !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = models.AddTeamMember(ctx.Org.Team, ctx.Doer.ID)
	case "leave":
		err = models.RemoveTeamMember(ctx.Org.Team, ctx.Doer.ID)
		if err != nil {
			if organization.IsErrLastOrgOwner(err) {
				ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			} else {
				log.Error("Action(%s): %v", ctx.Params(":action"), err)
				ctx.JSON(http.StatusOK, map[string]interface{}{
					"ok":  false,
					"err": err.Error(),
				})
				return
			}
		}
		ctx.JSON(http.StatusOK,
			map[string]interface{}{
				"redirect": ctx.Org.OrgLink + "/teams/",
			})
		return
	case "remove":
		if !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = models.RemoveTeamMember(ctx.Org.Team, uid)
		if err != nil {
			if organization.IsErrLastOrgOwner(err) {
				ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			} else {
				log.Error("Action(%s): %v", ctx.Params(":action"), err)
				ctx.JSON(http.StatusOK, map[string]interface{}{
					"ok":  false,
					"err": err.Error(),
				})
				return
			}
		}
		ctx.JSON(http.StatusOK,
			map[string]interface{}{
				"redirect": ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName),
			})
		return
	case "add":
		if !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		uname := utils.RemoveUsernameParameterSuffix(strings.ToLower(ctx.FormString("uname")))
		var u *user_model.User
		u, err = user_model.GetUserByName(ctx, uname)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
				ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
			} else {
				ctx.ServerError(" GetUserByName", err)
			}
			return
		}

		if u.IsOrganization() {
			ctx.Flash.Error(ctx.Tr("form.cannot_add_org_to_team"))
			ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
			return
		}

		if ctx.Org.Team.IsMember(u.ID) {
			ctx.Flash.Error(ctx.Tr("org.teams.add_duplicate_users"))
		} else {
			err = models.AddTeamMember(ctx.Org.Team, u.ID)
		}

		page = "team"
	}

	if err != nil {
		if organization.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
		} else {
			log.Error("Action(%s): %v", ctx.Params(":action"), err)
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"ok":  false,
				"err": err.Error(),
			})
			return
		}
	}

	switch page {
	case "team":
		ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
	case "home":
		ctx.Redirect(ctx.Org.Organization.AsUser().HomeLink())
	default:
		ctx.Redirect(ctx.Org.OrgLink + "/teams")
	}
}

// TeamsRepoAction operate team's repository
func TeamsRepoAction(ctx *context.Context) {
	if !ctx.Org.IsOwner {
		ctx.Error(http.StatusNotFound)
		return
	}

	var err error
	action := ctx.Params(":action")
	switch action {
	case "add":
		repoName := path.Base(ctx.FormString("repo_name"))
		var repo *repo_model.Repository
		repo, err = repo_model.GetRepositoryByName(ctx.Org.Organization.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				ctx.Flash.Error(ctx.Tr("org.teams.add_nonexistent_repo"))
				ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories")
				return
			}
			ctx.ServerError("GetRepositoryByName", err)
			return
		}
		err = models.AddRepository(ctx.Org.Team, repo)
	case "remove":
		err = models.RemoveRepository(ctx.Org.Team, ctx.FormInt64("repoid"))
	case "addall":
		err = models.AddAllRepositories(ctx.Org.Team)
	case "removeall":
		err = models.RemoveAllRepositories(ctx.Org.Team)
	}

	if err != nil {
		log.Error("Action(%s): '%s' %v", ctx.Params(":action"), ctx.Org.Team.Name, err)
		ctx.ServerError("TeamsRepoAction", err)
		return
	}

	if action == "addall" || action == "removeall" {
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"redirect": ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories",
		})
		return
	}
	ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories")
}

// NewTeam render create new team page
func NewTeam(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamsNew"] = true
	ctx.Data["Team"] = &organization.Team{}
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamNew)
}

func getUnitPerms(forms url.Values) map[unit_model.Type]perm.AccessMode {
	unitPerms := make(map[unit_model.Type]perm.AccessMode)
	for k, v := range forms {
		if strings.HasPrefix(k, "unit_") {
			t, _ := strconv.Atoi(k[5:])
			if t > 0 {
				vv, _ := strconv.Atoi(v[0])
				unitPerms[unit_model.Type(t)] = perm.AccessMode(vv)
			}
		}
	}
	return unitPerms
}

// NewTeamPost response for create new team
func NewTeamPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateTeamForm)
	includesAllRepositories := form.RepoAccess == "all"
	unitPerms := getUnitPerms(ctx.Req.Form)
	p := perm.ParseAccessMode(form.Permission)
	if p < perm.AccessModeAdmin {
		// if p is less than admin accessmode, then it should be general accessmode,
		// so we should calculate the minial accessmode from units accessmodes.
		p = unit_model.MinUnitAccessMode(unitPerms)
	}

	t := &organization.Team{
		OrgID:                   ctx.Org.Organization.ID,
		Name:                    form.TeamName,
		Description:             form.Description,
		AccessMode:              p,
		IncludesAllRepositories: includesAllRepositories,
		CanCreateOrgRepo:        form.CanCreateOrgRepo,
	}

	if t.AccessMode < perm.AccessModeAdmin {
		units := make([]*organization.TeamUnit, 0, len(unitPerms))
		for tp, perm := range unitPerms {
			units = append(units, &organization.TeamUnit{
				OrgID:      ctx.Org.Organization.ID,
				Type:       tp,
				AccessMode: perm,
			})
		}
		t.Units = units
	}

	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamsNew"] = true
	ctx.Data["Units"] = unit_model.Units
	ctx.Data["Team"] = t

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTeamNew)
		return
	}

	if t.AccessMode < perm.AccessModeAdmin && len(unitPerms) == 0 {
		ctx.RenderWithErr(ctx.Tr("form.team_no_units_error"), tplTeamNew, &form)
		return
	}

	if err := models.NewTeam(t); err != nil {
		ctx.Data["Err_TeamName"] = true
		switch {
		case organization.IsErrTeamAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("form.team_name_been_taken"), tplTeamNew, &form)
		default:
			ctx.ServerError("NewTeam", err)
		}
		return
	}
	log.Trace("Team created: %s/%s", ctx.Org.Organization.Name, t.Name)
	ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(t.LowerName))
}

// TeamMembers render team members page
func TeamMembers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Team.Name
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamMembers"] = true
	if err := ctx.Org.Team.GetMembersCtx(ctx); err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamMembers)
}

// TeamRepositories show the repositories of team
func TeamRepositories(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Team.Name
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamRepos"] = true
	if err := ctx.Org.Team.GetRepositoriesCtx(ctx); err != nil {
		ctx.ServerError("GetRepositories", err)
		return
	}
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamRepositories)
}

// SearchTeam api for searching teams
func SearchTeam(ctx *context.Context) {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}

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
		log.Error("convert ToTeams failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "SearchTeam failed to get units",
		})
		return
	}

	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": apiTeams,
	})
}

// EditTeam render team edit page
func EditTeam(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["team_name"] = ctx.Org.Team.Name
	ctx.Data["desc"] = ctx.Org.Team.Description
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamNew)
}

// EditTeamPost response for modify team information
func EditTeamPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateTeamForm)
	t := ctx.Org.Team
	unitPerms := getUnitPerms(ctx.Req.Form)
	isAuthChanged := false
	isIncludeAllChanged := false
	includesAllRepositories := form.RepoAccess == "all"

	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["Team"] = t
	ctx.Data["Units"] = unit_model.Units

	if !t.IsOwnerTeam() {
		// Validate permission level.
		newAccessMode := perm.ParseAccessMode(form.Permission)
		if newAccessMode < perm.AccessModeAdmin {
			// if p is less than admin accessmode, then it should be general accessmode,
			// so we should calculate the minial accessmode from units accessmodes.
			newAccessMode = unit_model.MinUnitAccessMode(unitPerms)
		}

		t.Name = form.TeamName
		if t.AccessMode != newAccessMode {
			isAuthChanged = true
			t.AccessMode = newAccessMode
		}

		if t.IncludesAllRepositories != includesAllRepositories {
			isIncludeAllChanged = true
			t.IncludesAllRepositories = includesAllRepositories
		}
	}
	t.Description = form.Description
	if t.AccessMode < perm.AccessModeAdmin {
		units := make([]organization.TeamUnit, 0, len(unitPerms))
		for tp, perm := range unitPerms {
			units = append(units, organization.TeamUnit{
				OrgID:      t.OrgID,
				TeamID:     t.ID,
				Type:       tp,
				AccessMode: perm,
			})
		}
		if err := organization.UpdateTeamUnits(t, units); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateTeamUnits", err.Error())
			return
		}
	}
	t.CanCreateOrgRepo = form.CanCreateOrgRepo

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTeamNew)
		return
	}

	if t.AccessMode < perm.AccessModeAdmin && len(unitPerms) == 0 {
		ctx.RenderWithErr(ctx.Tr("form.team_no_units_error"), tplTeamNew, &form)
		return
	}

	if err := models.UpdateTeam(t, isAuthChanged, isIncludeAllChanged); err != nil {
		ctx.Data["Err_TeamName"] = true
		switch {
		case organization.IsErrTeamAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("form.team_name_been_taken"), tplTeamNew, &form)
		default:
			ctx.ServerError("UpdateTeam", err)
		}
		return
	}
	ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(t.LowerName))
}

// DeleteTeam response for the delete team request
func DeleteTeam(ctx *context.Context) {
	if err := models.DeleteTeam(ctx.Org.Team); err != nil {
		ctx.Flash.Error("DeleteTeam: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("org.teams.delete_team_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Org.OrgLink + "/teams",
	})
}
