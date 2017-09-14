// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ProtectedBranch render the page to protect the repository
func ProtectedBranch(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsBranches"] = true

	protectedBranches, err := ctx.Repo.Repository.GetProtectedBranches()
	if err != nil {
		ctx.Handle(500, "GetProtectedBranches", err)
		return
	}
	ctx.Data["ProtectedBranches"] = protectedBranches

	branches := ctx.Data["Branches"].([]string)
	leftBranches := make([]string, 0, len(branches)-len(protectedBranches))
	for _, b := range branches {
		var protected bool
		for _, pb := range protectedBranches {
			if b == pb.BranchName {
				protected = true
				break
			}
		}
		if !protected {
			leftBranches = append(leftBranches, b)
		}
	}

	ctx.Data["LeftBranches"] = leftBranches

	ctx.HTML(200, tplBranches)
}

// ProtectedBranchPost response for protect for a branch of a repository
func ProtectedBranchPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsBranches"] = true

	repo := ctx.Repo.Repository

	switch ctx.Query("action") {
	case "default_branch":
		if ctx.HasError() {
			ctx.HTML(200, tplBranches)
			return
		}

		branch := ctx.Query("branch")
		if !ctx.Repo.GitRepo.IsBranchExist(branch) {
			ctx.Status(404)
			return
		} else if repo.DefaultBranch != branch {
			repo.DefaultBranch = branch
			if err := ctx.Repo.GitRepo.SetDefaultBranch(branch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					ctx.Handle(500, "SetDefaultBranch", err)
					return
				}
			}
			if err := repo.UpdateDefaultBranch(); err != nil {
				ctx.Handle(500, "SetDefaultBranch", err)
				return
			}
		}

		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
	default:
		ctx.Handle(404, "", nil)
	}
}

// SettingsProtectedBranch renders the protected branch setting page
func SettingsProtectedBranch(c *context.Context) {
	branch := c.Params("*")
	if !c.Repo.GitRepo.IsBranchExist(branch) {
		c.NotFound()
		return
	}

	c.Data["Title"] = c.Tr("repo.settings.protected_branches") + " - " + branch
	c.Data["PageIsSettingsBranches"] = true

	protectBranch, err := models.GetProtectedBranchBy(c.Repo.Repository.ID, branch)
	if err != nil {
		if !models.IsErrBranchNotExist(err) {
			c.Handle(500, "GetProtectBranchOfRepoByName", err)
			return
		}
	}

	if protectBranch == nil {
		// No options found, create defaults.
		protectBranch = &models.ProtectedBranch{
			BranchName: branch,
		}
	}

	users, err := c.Repo.Repository.GetWriters()
	if err != nil {
		c.Handle(500, "Repo.Repository.GetWriters", err)
		return
	}
	c.Data["Users"] = users
	c.Data["whitelist_users"] = strings.Join(base.Int64sToStrings(protectBranch.WhitelistUserIDs), ",")

	if c.Repo.Owner.IsOrganization() {
		teams, err := c.Repo.Owner.TeamsWithAccessToRepo(c.Repo.Repository.ID, models.AccessModeWrite)
		if err != nil {
			c.Handle(500, "Repo.Owner.TeamsWithAccessToRepo", err)
			return
		}
		c.Data["Teams"] = teams
		c.Data["whitelist_teams"] = strings.Join(base.Int64sToStrings(protectBranch.WhitelistTeamIDs), ",")
	}

	c.Data["Branch"] = protectBranch
	c.HTML(200, tplProtectedBranch)
}

// SettingsProtectedBranchPost updates the protected branch settings
func SettingsProtectedBranchPost(ctx *context.Context, f auth.ProtectBranchForm) {
	branch := ctx.Params("*")
	if !ctx.Repo.GitRepo.IsBranchExist(branch) {
		ctx.NotFound()
		return
	}

	protectBranch, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, branch)
	if err != nil {
		if !models.IsErrBranchNotExist(err) {
			ctx.Handle(500, "GetProtectBranchOfRepoByName", err)
			return
		}
	}

	if f.Protected {
		if protectBranch == nil {
			// No options found, create defaults.
			protectBranch = &models.ProtectedBranch{
				RepoID:     ctx.Repo.Repository.ID,
				BranchName: branch,
			}
		}

		protectBranch.EnableWhitelist = f.EnableWhitelist
		whitelistUsers, _ := base.StringsToInt64s(strings.Split(f.WhitelistUsers, ","))
		whitelistTeams, _ := base.StringsToInt64s(strings.Split(f.WhitelistTeams, ","))
		err = models.UpdateProtectBranch(ctx.Repo.Repository, protectBranch, whitelistUsers, whitelistTeams)
		if err != nil {
			ctx.Handle(500, "UpdateProtectBranch", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.update_protect_branch_success", branch))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches/%s", ctx.Repo.RepoLink, branch))
	} else {
		if protectBranch != nil {
			if err := ctx.Repo.Repository.DeleteProtectedBranch(protectBranch.ID); err != nil {
				ctx.Handle(500, "DeleteProtectedBranch", err)
				return
			}
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.remove_protected_branch_success", branch))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
	}
}
