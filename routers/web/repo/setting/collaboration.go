// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/mailer"
	repo_service "code.gitea.io/gitea/services/repository"
)

// Collaboration render a repository's collaboration page
func Collaboration(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.collaboration")
	ctx.Data["PageIsSettingsCollaboration"] = true

	users, _, err := repo_model.GetCollaborators(ctx, &repo_model.FindCollaborationOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("GetCollaborators", err)
		return
	}
	ctx.Data["Collaborators"] = users

	teams, err := organization.GetRepoTeams(ctx, ctx.Repo.Repository.OwnerID, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetRepoTeams", err)
		return
	}
	ctx.Data["Teams"] = teams
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["OrgID"] = ctx.Repo.Repository.OwnerID
	ctx.Data["OrgName"] = ctx.Repo.Repository.OwnerName
	ctx.Data["Org"] = ctx.Repo.Repository.Owner
	ctx.Data["Units"] = unit_model.Units

	ctx.HTML(http.StatusOK, tplCollaboration)
}

// CollaborationPost response for actions for a collaboration of a repository
func CollaborationPost(ctx *context.Context) {
	name := strings.ToLower(ctx.FormString("collaborator"))
	if len(name) == 0 || ctx.Repo.Owner.LowerName == name {
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	u, err := user_model.GetUserByName(ctx, name)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}

	if !u.IsActive {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_inactive_user"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	// Organization is not allowed to be added as a collaborator.
	if u.IsOrganization() {
		ctx.Flash.Error(ctx.Tr("repo.settings.org_not_allowed_to_be_collaborator"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	if got, err := repo_model.IsCollaborator(ctx, ctx.Repo.Repository.ID, u.ID); err == nil && got {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	// find the owner team of the organization the repo belongs too and
	// check if the user we're trying to add is an owner.
	if ctx.Repo.Repository.Owner.IsOrganization() {
		if isOwner, err := organization.IsOrganizationOwner(ctx, ctx.Repo.Repository.Owner.ID, u.ID); err != nil {
			ctx.ServerError("IsOrganizationOwner", err)
			return
		} else if isOwner {
			ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_owner"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
			return
		}
	}

	if err = repo_service.AddOrUpdateCollaborator(ctx, ctx.Repo.Repository, u, perm.AccessModeWrite); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator.blocked_user"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		} else {
			ctx.ServerError("AddOrUpdateCollaborator", err)
		}
		return
	}

	if setting.Service.EnableNotifyMail {
		mailer.SendCollaboratorMail(u, ctx.Doer, ctx.Repo.Repository)
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_collaborator_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

// ChangeCollaborationAccessMode response for changing access of a collaboration
func ChangeCollaborationAccessMode(ctx *context.Context) {
	if err := repo_model.ChangeCollaborationAccessMode(
		ctx,
		ctx.Repo.Repository,
		ctx.FormInt64("uid"),
		perm.AccessMode(ctx.FormInt("mode"))); err != nil {
		log.Error("ChangeCollaborationAccessMode: %v", err)
	}
}

// DeleteCollaboration delete a collaboration for a repository
func DeleteCollaboration(ctx *context.Context) {
	if collaborator, err := user_model.GetUserByID(ctx, ctx.FormInt64("id")); err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
		} else {
			ctx.ServerError("GetUserByName", err)
			return
		}
	} else {
		if err := repo_service.DeleteCollaboration(ctx, ctx.Repo.Repository, collaborator); err != nil {
			ctx.Flash.Error("DeleteCollaboration: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.remove_collaborator_success"))
		}
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/collaboration")
}

// AddTeamPost response for adding a team to a repository
func AddTeamPost(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	name := strings.ToLower(ctx.FormString("team"))
	if len(name) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := organization.OrgFromUser(ctx.Repo.Owner).GetTeam(ctx, name)
	if err != nil {
		if organization.IsErrTeamNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.team_not_exist"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		} else {
			ctx.ServerError("GetTeam", err)
		}
		return
	}

	if team.OrgID != ctx.Repo.Repository.OwnerID {
		ctx.Flash.Error(ctx.Tr("repo.settings.team_not_in_organization"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if organization.HasTeamRepo(ctx, ctx.Repo.Repository.OwnerID, team.ID, ctx.Repo.Repository.ID) {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_team_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if err = repo_service.TeamAddRepository(ctx, team, ctx.Repo.Repository); err != nil {
		ctx.ServerError("TeamAddRepository", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_team_success"))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
}

// DeleteTeam response for deleting a team from a repository
func DeleteTeam(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := organization.GetTeamByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.ServerError("GetTeamByID", err)
		return
	}

	if err = repo_service.RemoveRepositoryFromTeam(ctx, team, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("team.RemoveRepositorys", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.remove_team_success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/collaboration")
}
