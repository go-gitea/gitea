// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

// GetBranch get a branch of a repository
func GetBranch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches/{branch} repository repoGetBranch
	// ---
	// summary: Retrieve a specific branch from a repository
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
	// - name: branch
	//   in: path
	//   description: branch to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Branch"
	if ctx.Repo.TreePath != "" {
		// if TreePath != "", then URL contained extra slashes
		// (i.e. "master/subbranch" instead of "master"), so branch does
		// not exist
		ctx.NotFound()
		return
	}
	branchName := ctx.Repo.BranchName
	branch, err := ctx.Repo.Repository.GetBranch(branchName)
	if err != nil {
		if git.IsErrBranchNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranch", err)
		}
		return
	}

	protected, err := ctx.Repo.Repository.IsProtectedBranch(branchName, ctx.Repo.Owner)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsProtectedBranch", err)
		return
	}

	c, err := branch.GetCommit()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToBranch(ctx.Repo.Repository, branch, c, protected))
}

// ListBranches list all the branches of a repository
func ListBranches(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches repository repoListBranches
	// ---
	// summary: List a repository's branches
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
	//     "$ref": "#/responses/BranchList"
	branches, err := ctx.Repo.Repository.GetBranches()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranches", err)
		return
	}

	apiBranches := make([]*api.Branch, len(branches))
	for i := range branches {
		c, err := branches[i].GetCommit()
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommit", err)
			return
		}

		protected, err := ctx.Repo.Repository.IsProtectedBranch(branches[i].Name, ctx.Repo.Owner)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsProtectedBranch", err)
			return
		}

		apiBranches[i] = convert.ToBranch(ctx.Repo.Repository, branches[i], c, protected)
	}

	ctx.JSON(http.StatusOK, &apiBranches)
}

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches/{branch}/protection repository repoGetBranchProtection
	// ---
	// summary: Retrieve a specific branch protection from a repository
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
	// - name: branch
	//   in: path
	//   description: branch to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Branch"
	protectBranch, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, ctx.Params(":branch"))
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetProtectedBranchBy", err)
			return
		}
	}
	ctx.JSON(http.StatusOK, protectBranch)
}

// UpdateProtectBranch saves branch protection options of repository.
// If ID is 0, it creates a new record. Otherwise, updates existing record.
// This function also performs check if whitelist user and team's IDs have been changed
// to avoid unnecessary whitelist delete and regenerate.
func UpdateProtectBranch(ctx *context.APIContext, f auth.ProtectBranchForm) {
	// swagger:operation PUT /repos/{owner}/{repo}/branches/{branch}/protection repository repoUpdateProtectBranch
	// ---
	// summary: Update branch protection of a repository
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
	// - name: branch
	//   in: path
	//   description: branch to update
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/ProtectBranchForm"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Branch"
	branch := ctx.Params(":branch")
	protectBranch, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, branch)
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetProtectedBranchBy", err)
			return
		}
	}

	if f.Protected {
		if protectBranch == nil {
			protectBranch = &models.ProtectedBranch{
				RepoID:     ctx.Repo.Repository.ID,
				BranchName: branch,
			}
		}

		var whitelistUsers, whitelistTeams, mergeWhitelistUsers, mergeWhitelistTeams, approvalsWhitelistUsers, approvalsWhitelistTeams []int64
		protectBranch.EnableWhitelist = f.EnableWhitelist
		if strings.TrimSpace(f.WhitelistUsers) != "" {
			whitelistUsers, _ = base.StringsToInt64s(strings.Split(f.WhitelistUsers, ","))
		}
		if strings.TrimSpace(f.WhitelistTeams) != "" {
			whitelistTeams, _ = base.StringsToInt64s(strings.Split(f.WhitelistTeams, ","))
		}
		protectBranch.EnableMergeWhitelist = f.EnableMergeWhitelist
		if strings.TrimSpace(f.MergeWhitelistUsers) != "" {
			mergeWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistUsers, ","))
		}
		if strings.TrimSpace(f.MergeWhitelistTeams) != "" {
			mergeWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistTeams, ","))
		}
		protectBranch.RequiredApprovals = f.RequiredApprovals
		if strings.TrimSpace(f.ApprovalsWhitelistUsers) != "" {
			approvalsWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistUsers, ","))
		}
		if strings.TrimSpace(f.ApprovalsWhitelistTeams) != "" {
			approvalsWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistTeams, ","))
		}
		if err = models.UpdateProtectBranch(ctx.Repo.Repository, protectBranch, models.WhitelistOptions{
			UserIDs:          whitelistUsers,
			TeamIDs:          whitelistTeams,
			MergeUserIDs:     mergeWhitelistUsers,
			MergeTeamIDs:     mergeWhitelistTeams,
			ApprovalsUserIDs: approvalsWhitelistUsers,
			ApprovalsTeamIDs: approvalsWhitelistTeams,
		}); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateProtectBranch", err)
			return
		}
	} else if protectBranch != nil {
		if err := ctx.Repo.Repository.DeleteProtectedBranch(protectBranch.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "DeleteProtectedBranch", err)
			return
		}
	}
	ctx.JSON(http.StatusOK, protectBranch)
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func DeleteProtectedBranch(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/branches/{branch}/protection repository repoDeleteProtectedBranch
	// ---
	// summary: Remove branch protection from a repository
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
	// - name: branch
	//   in: path
	//   description: branch to remove protection
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	branchName := ctx.Params(":branch")
	protected, err := ctx.Repo.Repository.IsProtectedBranch(branchName, ctx.Repo.Owner)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsProtectedBranch", err)
		return
	}
	if !protected {
		ctx.Status(http.StatusNoContent)
		return
	}
	protectBranch, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, branchName)
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetProtectedBranchBy", err)
			return
		}
	}
	if err = ctx.Repo.Repository.DeleteProtectedBranch(protectBranch.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProtectedBranch", err)
	}
	ctx.Status(http.StatusNoContent)
}
