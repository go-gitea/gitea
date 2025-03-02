// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	org_service "code.gitea.io/gitea/services/org"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	// tplTeams template path for teams list page
	tplTeams templates.TplName = "org/team/teams"
	// tplTeamNew template path for create new team page
	tplTeamNew templates.TplName = "org/team/new"
	// tplTeamMembers template path for showing team members page
	tplTeamMembers templates.TplName = "org/team/members"
	// tplTeamRepositories template path for showing team repositories page
	tplTeamRepositories templates.TplName = "org/team/repositories"
	// tplTeamInvite template path for team invites page
	tplTeamInvite templates.TplName = "org/team/invite"
)

// Teams render teams list page
func Teams(ctx *context.Context) {
	org := ctx.Org.Organization
	ctx.Data["Title"] = org.FullName
	ctx.Data["PageIsOrgTeams"] = true

	for _, t := range ctx.Org.Teams {
		if err := t.LoadMembers(ctx); err != nil {
			ctx.ServerError("GetMembers", err)
			return
		}
	}
	ctx.Data["Teams"] = ctx.Org.Teams

	_, err := shared_user.PrepareOrgHeader(ctx)
	if err != nil {
		ctx.ServerError("PrepareOrgHeader", err)
		return
	}

	ctx.HTML(http.StatusOK, tplTeams)
}

// TeamsAction response for join, leave, remove, add operations to team
func TeamsAction(ctx *context.Context) {
	page := ctx.FormString("page")
	var err error
	switch ctx.PathParam("action") {
	case "join":
		if !ctx.Org.IsOwner {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
		err = org_service.AddTeamMember(ctx, ctx.Org.Team, ctx.Doer)
	case "leave":
		err = org_service.RemoveTeamMember(ctx, ctx.Org.Team, ctx.Doer)
		if err != nil {
			if org_model.IsErrLastOrgOwner(err) {
				ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			} else {
				log.Error("Action(%s): %v", ctx.PathParam("action"), err)
				ctx.JSON(http.StatusOK, map[string]any{
					"ok":  false,
					"err": err.Error(),
				})
				return
			}
		}
		checkIsOrgMemberAndRedirect(ctx, ctx.Org.OrgLink+"/teams/")
		return
	case "remove":
		if !ctx.Org.IsOwner {
			ctx.HTTPError(http.StatusNotFound)
			return
		}

		user, _ := user_model.GetUserByID(ctx, ctx.FormInt64("uid"))
		if user == nil {
			ctx.Redirect(ctx.Org.OrgLink + "/teams")
			return
		}

		err = org_service.RemoveTeamMember(ctx, ctx.Org.Team, user)
		if err != nil {
			if org_model.IsErrLastOrgOwner(err) {
				ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			} else {
				log.Error("Action(%s): %v", ctx.PathParam("action"), err)
				ctx.JSON(http.StatusOK, map[string]any{
					"ok":  false,
					"err": err.Error(),
				})
				return
			}
		}
		checkIsOrgMemberAndRedirect(ctx, ctx.Org.OrgLink+"/teams/"+url.PathEscape(ctx.Org.Team.LowerName))
		return
	case "add":
		if !ctx.Org.IsOwner {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
		uname := strings.ToLower(ctx.FormString("uname"))
		var u *user_model.User
		u, err = user_model.GetUserByName(ctx, uname)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				if setting.MailService != nil && user_model.ValidateEmail(uname) == nil {
					if err := org_service.CreateTeamInvite(ctx, ctx.Doer, ctx.Org.Team, uname); err != nil {
						if org_model.IsErrTeamInviteAlreadyExist(err) {
							ctx.Flash.Error(ctx.Tr("form.duplicate_invite_to_team"))
						} else if org_model.IsErrUserEmailAlreadyAdded(err) {
							ctx.Flash.Error(ctx.Tr("org.teams.add_duplicate_users"))
						} else {
							ctx.ServerError("CreateTeamInvite", err)
							return
						}
					}
				} else {
					ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
				}
				ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return
		}

		if u.IsOrganization() {
			ctx.Flash.Error(ctx.Tr("form.cannot_add_org_to_team"))
			ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
			return
		}

		if ctx.Org.Team.IsMember(ctx, u.ID) {
			ctx.Flash.Error(ctx.Tr("org.teams.add_duplicate_users"))
		} else {
			err = org_service.AddTeamMember(ctx, ctx.Org.Team, u)
		}

		page = "team"
	case "remove_invite":
		if !ctx.Org.IsOwner {
			ctx.HTTPError(http.StatusNotFound)
			return
		}

		iid := ctx.FormInt64("iid")
		if iid == 0 {
			ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName))
			return
		}

		if err := org_model.RemoveInviteByID(ctx, iid, ctx.Org.Team.ID); err != nil {
			log.Error("Action(%s): %v", ctx.PathParam("action"), err)
			ctx.ServerError("RemoveInviteByID", err)
			return
		}

		page = "team"
	}

	if err != nil {
		if org_model.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Flash.Error(ctx.Tr("org.teams.members.blocked_user"))
		} else {
			log.Error("Action(%s): %v", ctx.PathParam("action"), err)
			ctx.JSON(http.StatusOK, map[string]any{
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

func checkIsOrgMemberAndRedirect(ctx *context.Context, defaultRedirect string) {
	if isOrgMember, err := org_model.IsOrganizationMember(ctx, ctx.Org.Organization.ID, ctx.Doer.ID); err != nil {
		ctx.ServerError("IsOrganizationMember", err)
		return
	} else if !isOrgMember {
		if ctx.Org.Organization.Visibility.IsPrivate() {
			defaultRedirect = setting.AppSubURL + "/"
		} else {
			defaultRedirect = ctx.Org.Organization.HomeLink()
		}
	}
	ctx.JSONRedirect(defaultRedirect)
}

// TeamsRepoAction operate team's repository
func TeamsRepoAction(ctx *context.Context) {
	if !ctx.Org.IsOwner {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	var err error
	action := ctx.PathParam("action")
	switch action {
	case "add":
		repoName := path.Base(ctx.FormString("repo_name"))
		var repo *repo_model.Repository
		repo, err = repo_model.GetRepositoryByName(ctx, ctx.Org.Organization.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				ctx.Flash.Error(ctx.Tr("org.teams.add_nonexistent_repo"))
				ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories")
				return
			}
			ctx.ServerError("GetRepositoryByName", err)
			return
		}
		err = repo_service.TeamAddRepository(ctx, ctx.Org.Team, repo)
	case "remove":
		err = repo_service.RemoveRepositoryFromTeam(ctx, ctx.Org.Team, ctx.FormInt64("repoid"))
	case "addall":
		err = repo_service.AddAllRepositoriesToTeam(ctx, ctx.Org.Team)
	case "removeall":
		err = repo_service.RemoveAllRepositoriesFromTeam(ctx, ctx.Org.Team)
	}

	if err != nil {
		log.Error("Action(%s): '%s' %v", ctx.PathParam("action"), ctx.Org.Team.Name, err)
		ctx.ServerError("TeamsRepoAction", err)
		return
	}

	if action == "addall" || action == "removeall" {
		ctx.JSONRedirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories")
		return
	}
	ctx.Redirect(ctx.Org.OrgLink + "/teams/" + url.PathEscape(ctx.Org.Team.LowerName) + "/repositories")
}

// NewTeam render create new team page
func NewTeam(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamsNew"] = true
	ctx.Data["Team"] = &org_model.Team{}
	ctx.Data["Units"] = unit_model.Units
	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}
	ctx.HTML(http.StatusOK, tplTeamNew)
}

func getUnitPerms(forms url.Values, teamPermission perm.AccessMode) map[unit_model.Type]perm.AccessMode {
	unitPerms := make(map[unit_model.Type]perm.AccessMode)
	for _, ut := range unit_model.AllRepoUnitTypes {
		// Default accessmode is none
		unitPerms[ut] = perm.AccessModeNone

		v, ok := forms[fmt.Sprintf("unit_%d", ut)]
		if ok {
			vv, _ := strconv.Atoi(v[0])
			if teamPermission >= perm.AccessModeAdmin {
				unitPerms[ut] = teamPermission
				// Don't allow `TypeExternal{Tracker,Wiki}` to influence this as they can only be set to READ perms.
				if ut == unit_model.TypeExternalTracker || ut == unit_model.TypeExternalWiki {
					unitPerms[ut] = perm.AccessModeRead
				}
			} else {
				unitPerms[ut] = perm.AccessMode(vv)
				if unitPerms[ut] >= perm.AccessModeAdmin {
					unitPerms[ut] = perm.AccessModeWrite
				}
			}
		}
	}
	return unitPerms
}

// NewTeamPost response for create new team
func NewTeamPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateTeamForm)
	includesAllRepositories := form.RepoAccess == "all"
	p := perm.ParseAccessMode(form.Permission)
	unitPerms := getUnitPerms(ctx.Req.Form, p)
	if p < perm.AccessModeAdmin {
		// if p is less than admin accessmode, then it should be general accessmode,
		// so we should calculate the minial accessmode from units accessmodes.
		p = unit_model.MinUnitAccessMode(unitPerms)
	}

	t := &org_model.Team{
		OrgID:                   ctx.Org.Organization.ID,
		Name:                    form.TeamName,
		Description:             form.Description,
		AccessMode:              p,
		IncludesAllRepositories: includesAllRepositories,
		CanCreateOrgRepo:        form.CanCreateOrgRepo,
	}

	units := make([]*org_model.TeamUnit, 0, len(unitPerms))
	for tp, perm := range unitPerms {
		units = append(units, &org_model.TeamUnit{
			OrgID:      ctx.Org.Organization.ID,
			Type:       tp,
			AccessMode: perm,
		})
	}
	t.Units = units

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

	if err := org_service.NewTeam(ctx, t); err != nil {
		ctx.Data["Err_TeamName"] = true
		switch {
		case org_model.IsErrTeamAlreadyExist(err):
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

	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	if err := ctx.Org.Team.LoadMembers(ctx); err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	ctx.Data["Units"] = unit_model.Units

	invites, err := org_model.GetInvitesByTeamID(ctx, ctx.Org.Team.ID)
	if err != nil {
		ctx.ServerError("GetInvitesByTeamID", err)
		return
	}
	ctx.Data["Invites"] = invites
	ctx.Data["IsEmailInviteEnabled"] = setting.MailService != nil

	ctx.HTML(http.StatusOK, tplTeamMembers)
}

// TeamRepositories show the repositories of team
func TeamRepositories(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Team.Name
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["PageIsOrgTeamRepos"] = true

	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	repos, err := repo_model.GetTeamRepositories(ctx, &repo_model.SearchTeamRepoOptions{
		TeamID: ctx.Org.Team.ID,
	})
	if err != nil {
		ctx.ServerError("GetTeamRepositories", err)
		return
	}
	ctx.Data["Units"] = unit_model.Units
	ctx.Data["TeamRepos"] = repos
	ctx.HTML(http.StatusOK, tplTeamRepositories)
}

// SearchTeam api for searching teams
func SearchTeam(ctx *context.Context) {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}

	opts := &org_model.SearchTeamOptions{
		// UserID is not set because the router already requires the doer to be an org admin. Thus, we don't need to restrict to teams that the user belongs in
		Keyword:     ctx.FormTrim("q"),
		OrgID:       ctx.Org.Organization.ID,
		IncludeDesc: ctx.FormString("include_desc") == "" || ctx.FormBool("include_desc"),
		ListOptions: listOptions,
	}

	teams, maxResults, err := org_model.SearchTeam(ctx, opts)
	if err != nil {
		log.Error("SearchTeam failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "SearchTeam internal failure",
		})
		return
	}

	apiTeams, err := convert.ToTeams(ctx, teams, false)
	if err != nil {
		log.Error("convert ToTeams failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "SearchTeam failed to get units",
		})
		return
	}

	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": apiTeams,
	})
}

// EditTeam render team edit page
func EditTeam(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	if err := ctx.Org.Team.LoadUnits(ctx); err != nil {
		ctx.ServerError("LoadUnits", err)
		return
	}
	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}
	ctx.Data["Team"] = ctx.Org.Team
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamNew)
}

// EditTeamPost response for modify team information
func EditTeamPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateTeamForm)
	t := ctx.Org.Team
	newAccessMode := perm.ParseAccessMode(form.Permission)
	unitPerms := getUnitPerms(ctx.Req.Form, newAccessMode)
	if newAccessMode < perm.AccessModeAdmin {
		// if newAccessMode is less than admin accessmode, then it should be general accessmode,
		// so we should calculate the minial accessmode from units accessmodes.
		newAccessMode = unit_model.MinUnitAccessMode(unitPerms)
	}
	isAuthChanged := false
	isIncludeAllChanged := false
	includesAllRepositories := form.RepoAccess == "all"

	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsOrgTeams"] = true
	ctx.Data["Team"] = t
	ctx.Data["Units"] = unit_model.Units

	if !t.IsOwnerTeam() {
		t.Name = form.TeamName
		if t.AccessMode != newAccessMode {
			isAuthChanged = true
			t.AccessMode = newAccessMode
		}

		if t.IncludesAllRepositories != includesAllRepositories {
			isIncludeAllChanged = true
			t.IncludesAllRepositories = includesAllRepositories
		}
		t.CanCreateOrgRepo = form.CanCreateOrgRepo
	} else {
		t.CanCreateOrgRepo = true
	}

	t.Description = form.Description
	units := make([]*org_model.TeamUnit, 0, len(unitPerms))
	for tp, perm := range unitPerms {
		units = append(units, &org_model.TeamUnit{
			OrgID:      t.OrgID,
			TeamID:     t.ID,
			Type:       tp,
			AccessMode: perm,
		})
	}
	t.Units = units

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTeamNew)
		return
	}

	if t.AccessMode < perm.AccessModeAdmin && len(unitPerms) == 0 {
		ctx.RenderWithErr(ctx.Tr("form.team_no_units_error"), tplTeamNew, &form)
		return
	}

	if err := org_service.UpdateTeam(ctx, t, isAuthChanged, isIncludeAllChanged); err != nil {
		ctx.Data["Err_TeamName"] = true
		switch {
		case org_model.IsErrTeamAlreadyExist(err):
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
	if err := org_service.DeleteTeam(ctx, ctx.Org.Team); err != nil {
		ctx.Flash.Error("DeleteTeam: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("org.teams.delete_team_success"))
	}

	ctx.JSONRedirect(ctx.Org.OrgLink + "/teams")
}

// TeamInvite renders the team invite page
func TeamInvite(ctx *context.Context) {
	invite, org, team, inviter, err := getTeamInviteFromContext(ctx)
	if err != nil {
		if org_model.IsErrTeamInviteNotFound(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("getTeamInviteFromContext", err)
		}
		return
	}

	ctx.Data["Title"] = ctx.Tr("org.teams.invite_team_member", team.Name)
	ctx.Data["Invite"] = invite
	ctx.Data["Organization"] = org
	ctx.Data["Team"] = team
	ctx.Data["Inviter"] = inviter

	ctx.HTML(http.StatusOK, tplTeamInvite)
}

// TeamInvitePost handles the team invitation
func TeamInvitePost(ctx *context.Context) {
	invite, org, team, _, err := getTeamInviteFromContext(ctx)
	if err != nil {
		if org_model.IsErrTeamInviteNotFound(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("getTeamInviteFromContext", err)
		}
		return
	}

	if err := org_service.AddTeamMember(ctx, team, ctx.Doer); err != nil {
		ctx.ServerError("AddTeamMember", err)
		return
	}

	if err := org_model.RemoveInviteByID(ctx, invite.ID, team.ID); err != nil {
		log.Error("RemoveInviteByID: %v", err)
	}

	ctx.Redirect(org.OrganisationLink() + "/teams/" + url.PathEscape(team.LowerName))
}

func getTeamInviteFromContext(ctx *context.Context) (*org_model.TeamInvite, *org_model.Organization, *org_model.Team, *user_model.User, error) {
	invite, err := org_model.GetInviteByToken(ctx, ctx.PathParam("token"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	inviter, err := user_model.GetUserByID(ctx, invite.InviterID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	team, err := org_model.GetTeamByID(ctx, invite.TeamID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	org, err := user_model.GetUserByID(ctx, team.OrgID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return invite, org_model.OrgFromUser(org), team, inviter, nil
}
