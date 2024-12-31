// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	pull_service "code.gitea.io/gitea/services/pull"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
)

// GetBranch get a branch of a repository
func GetBranch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches/{branch} repository repoGetBranch
	// ---
	// summary: Retrieve a specific branch from a repository, including its effective branch protection
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	branchName := ctx.PathParam("*")

	branch, err := ctx.Repo.GitRepo.GetBranch(branchName)
	if err != nil {
		if git.IsErrBranchNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranch", err)
		}
		return
	}

	c, err := branch.GetCommit()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit", err)
		return
	}

	branchProtection, err := git_model.GetFirstMatchProtectedBranchRule(ctx, ctx.Repo.Repository.ID, branchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranchProtection", err)
		return
	}

	br, err := convert.ToBranch(ctx, ctx.Repo.Repository, branch.Name, c, branchProtection, ctx.Doer, ctx.Repo.IsAdmin())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
		return
	}

	ctx.JSON(http.StatusOK, br)
}

// DeleteBranch get a branch of a repository
func DeleteBranch(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/branches/{branch} repository repoDeleteBranch
	// ---
	// summary: Delete a specific branch from a repository
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
	//   description: branch to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusNotFound, "", "Git Repository is empty.")
		return
	}

	if ctx.Repo.Repository.IsMirror {
		ctx.Error(http.StatusForbidden, "", "Git Repository is a mirror.")
		return
	}

	branchName := ctx.PathParam("*")

	// check whether branches of this repository has been synced
	totalNumOfBranches, err := db.Count[git_model.Branch](ctx, git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountBranches", err)
		return
	}
	if totalNumOfBranches == 0 { // sync branches immediately because non-empty repository should have at least 1 branch
		_, err = repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0)
		if err != nil {
			ctx.ServerError("SyncRepoBranches", err)
			return
		}
	}

	if err := repo_service.DeleteBranch(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, branchName); err != nil {
		switch {
		case git.IsErrBranchNotExist(err):
			ctx.NotFound(err)
		case errors.Is(err, repo_service.ErrBranchIsDefault):
			ctx.Error(http.StatusForbidden, "DefaultBranch", fmt.Errorf("can not delete default branch"))
		case errors.Is(err, git_model.ErrBranchIsProtected):
			ctx.Error(http.StatusForbidden, "IsProtectedBranch", fmt.Errorf("branch protected"))
		default:
			ctx.Error(http.StatusInternalServerError, "DeleteBranch", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateBranch creates a branch for a user's repository
func CreateBranch(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/branches repository repoCreateBranch
	// ---
	// summary: Create a branch
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateBranchRepoOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Branch"
	//   "403":
	//     description: The branch is archived or a mirror.
	//   "404":
	//     description: The old branch does not exist.
	//   "409":
	//     description: The branch with the same name already exists.
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusNotFound, "", "Git Repository is empty.")
		return
	}

	if ctx.Repo.Repository.IsMirror {
		ctx.Error(http.StatusForbidden, "", "Git Repository is a mirror.")
		return
	}

	opt := web.GetForm(ctx).(*api.CreateBranchRepoOption)

	var oldCommit *git.Commit
	var err error

	if len(opt.OldRefName) > 0 {
		oldCommit, err = ctx.Repo.GitRepo.GetCommit(opt.OldRefName)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommit", err)
			return
		}
	} else if len(opt.OldBranchName) > 0 { //nolint
		if ctx.Repo.GitRepo.IsBranchExist(opt.OldBranchName) { //nolint
			oldCommit, err = ctx.Repo.GitRepo.GetBranchCommit(opt.OldBranchName) //nolint
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
				return
			}
		} else {
			ctx.Error(http.StatusNotFound, "", "The old branch does not exist")
			return
		}
	} else {
		oldCommit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
			return
		}
	}

	err = repo_service.CreateNewBranchFromCommit(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, oldCommit.ID.String(), opt.BranchName)
	if err != nil {
		if git_model.IsErrBranchNotExist(err) {
			ctx.Error(http.StatusNotFound, "", "The old branch does not exist")
		} else if release_service.IsErrTagAlreadyExists(err) {
			ctx.Error(http.StatusConflict, "", "The branch with the same tag already exists.")
		} else if git_model.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err) {
			ctx.Error(http.StatusConflict, "", "The branch already exists.")
		} else if git_model.IsErrBranchNameConflict(err) {
			ctx.Error(http.StatusConflict, "", "The branch with the same name already exists.")
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateNewBranchFromCommit", err)
		}
		return
	}

	branch, err := ctx.Repo.GitRepo.GetBranch(opt.BranchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranch", err)
		return
	}

	commit, err := branch.GetCommit()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit", err)
		return
	}

	branchProtection, err := git_model.GetFirstMatchProtectedBranchRule(ctx, ctx.Repo.Repository.ID, branch.Name)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranchProtection", err)
		return
	}

	br, err := convert.ToBranch(ctx, ctx.Repo.Repository, branch.Name, commit, branchProtection, ctx.Doer, ctx.Repo.IsAdmin())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
		return
	}

	ctx.JSON(http.StatusCreated, br)
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
	//     "$ref": "#/responses/BranchList"

	var totalNumOfBranches int64
	var apiBranches []*api.Branch

	listOptions := utils.GetListOptions(ctx)

	if !ctx.Repo.Repository.IsEmpty {
		if ctx.Repo.GitRepo == nil {
			ctx.Error(http.StatusInternalServerError, "Load git repository failed", nil)
			return
		}

		branchOpts := git_model.FindBranchOptions{
			ListOptions:     listOptions,
			RepoID:          ctx.Repo.Repository.ID,
			IsDeletedBranch: optional.Some(false),
		}
		var err error
		totalNumOfBranches, err = db.Count[git_model.Branch](ctx, branchOpts)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CountBranches", err)
			return
		}
		if totalNumOfBranches == 0 { // sync branches immediately because non-empty repository should have at least 1 branch
			totalNumOfBranches, err = repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0)
			if err != nil {
				ctx.ServerError("SyncRepoBranches", err)
				return
			}
		}

		rules, err := git_model.FindRepoProtectedBranchRules(ctx, ctx.Repo.Repository.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "FindMatchedProtectedBranchRules", err)
			return
		}

		branches, err := db.Find[git_model.Branch](ctx, branchOpts)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetBranches", err)
			return
		}

		apiBranches = make([]*api.Branch, 0, len(branches))
		for i := range branches {
			c, err := ctx.Repo.GitRepo.GetBranchCommit(branches[i].Name)
			if err != nil {
				// Skip if this branch doesn't exist anymore.
				if git.IsErrNotExist(err) {
					totalNumOfBranches--
					continue
				}
				ctx.Error(http.StatusInternalServerError, "GetCommit", err)
				return
			}

			branchProtection := rules.GetFirstMatched(branches[i].Name)
			apiBranch, err := convert.ToBranch(ctx, ctx.Repo.Repository, branches[i].Name, c, branchProtection, ctx.Doer, ctx.Repo.IsAdmin())
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
				return
			}
			apiBranches = append(apiBranches, apiBranch)
		}
	}

	ctx.SetLinkHeader(int(totalNumOfBranches), listOptions.PageSize)
	ctx.SetTotalCountHeader(totalNumOfBranches)
	ctx.JSON(http.StatusOK, apiBranches)
}

// UpdateBranch updates a repository's branch.
func UpdateBranch(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/branches/{branch} repository repoUpdateBranch
	// ---
	// summary: Update a branch
	// consumes:
	// - application/json
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
	//   description: name of the branch
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateBranchRepoOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opt := web.GetForm(ctx).(*api.UpdateBranchRepoOption)

	oldName := ctx.PathParam("*")
	repo := ctx.Repo.Repository

	if repo.IsEmpty {
		ctx.Error(http.StatusNotFound, "", "Git Repository is empty.")
		return
	}

	if repo.IsMirror {
		ctx.Error(http.StatusForbidden, "", "Git Repository is a mirror.")
		return
	}

	msg, err := repo_service.RenameBranch(ctx, repo, ctx.Doer, ctx.Repo.GitRepo, oldName, opt.Name)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "RenameBranch", err)
		return
	}
	if msg == "target_exist" {
		ctx.Error(http.StatusUnprocessableEntity, "", "Cannot rename a branch using the same name or rename to a branch that already exists.")
		return
	}
	if msg == "from_not_exist" {
		ctx.Error(http.StatusNotFound, "", "Branch doesn't exist.")
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetBranchProtection gets a branch protection
func GetBranchProtection(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branch_protections/{name} repository repoGetBranchProtection
	// ---
	// summary: Get a specific branch protection for the repository
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
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	bpName := ctx.PathParam("name")
	bp, err := git_model.GetProtectedBranchRuleByName(ctx, repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	ctx.JSON(http.StatusOK, convert.ToBranchProtection(ctx, bp, repo))
}

// ListBranchProtections list branch protections for a repo
func ListBranchProtections(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branch_protections repository repoListBranchProtection
	// ---
	// summary: List branch protections for a repository
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
	//     "$ref": "#/responses/BranchProtectionList"

	repo := ctx.Repo.Repository
	bps, err := git_model.FindRepoProtectedBranchRules(ctx, repo.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranches", err)
		return
	}
	apiBps := make([]*api.BranchProtection, len(bps))
	for i := range bps {
		apiBps[i] = convert.ToBranchProtection(ctx, bps[i], repo)
	}

	ctx.JSON(http.StatusOK, apiBps)
}

// CreateBranchProtection creates a branch protection for a repo
func CreateBranchProtection(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/branch_protections repository repoCreateBranchProtection
	// ---
	// summary: Create a branch protections for a repository
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateBranchProtectionOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/BranchProtection"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateBranchProtectionOption)
	repo := ctx.Repo.Repository

	ruleName := form.RuleName
	if ruleName == "" {
		ruleName = form.BranchName //nolint
	}
	if len(ruleName) == 0 {
		ctx.Error(http.StatusBadRequest, "both rule_name and branch_name are empty", "both rule_name and branch_name are empty")
		return
	}

	isPlainRule := !git_model.IsRuleNameSpecial(ruleName)
	var isBranchExist bool
	if isPlainRule {
		isBranchExist = git.IsBranchExist(ctx.Req.Context(), ctx.Repo.Repository.RepoPath(), ruleName)
	}

	protectBranch, err := git_model.GetProtectedBranchRuleByName(ctx, repo.ID, ruleName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectBranchOfRepoByName", err)
		return
	} else if protectBranch != nil {
		ctx.Error(http.StatusForbidden, "Create branch protection", "Branch protection already exist")
		return
	}

	var requiredApprovals int64
	if form.RequiredApprovals > 0 {
		requiredApprovals = form.RequiredApprovals
	}

	whitelistUsers, err := user_model.GetUserIDsByNames(ctx, form.PushWhitelistUsernames, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	forcePushAllowlistUsers, err := user_model.GetUserIDsByNames(ctx, form.ForcePushAllowlistUsernames, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	mergeWhitelistUsers, err := user_model.GetUserIDsByNames(ctx, form.MergeWhitelistUsernames, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	approvalsWhitelistUsers, err := user_model.GetUserIDsByNames(ctx, form.ApprovalsWhitelistUsernames, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	var whitelistTeams, forcePushAllowlistTeams, mergeWhitelistTeams, approvalsWhitelistTeams []int64
	if repo.Owner.IsOrganization() {
		whitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.PushWhitelistTeams, false)
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
		forcePushAllowlistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.ForcePushAllowlistTeams, false)
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
		mergeWhitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.MergeWhitelistTeams, false)
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
		approvalsWhitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.ApprovalsWhitelistTeams, false)
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
	}

	protectBranch = &git_model.ProtectedBranch{
		RepoID:                        ctx.Repo.Repository.ID,
		RuleName:                      ruleName,
		Priority:                      form.Priority,
		CanPush:                       form.EnablePush,
		EnableWhitelist:               form.EnablePush && form.EnablePushWhitelist,
		WhitelistDeployKeys:           form.EnablePush && form.EnablePushWhitelist && form.PushWhitelistDeployKeys,
		CanForcePush:                  form.EnablePush && form.EnableForcePush,
		EnableForcePushAllowlist:      form.EnablePush && form.EnableForcePush && form.EnableForcePushAllowlist,
		ForcePushAllowlistDeployKeys:  form.EnablePush && form.EnableForcePush && form.EnableForcePushAllowlist && form.ForcePushAllowlistDeployKeys,
		EnableMergeWhitelist:          form.EnableMergeWhitelist,
		EnableStatusCheck:             form.EnableStatusCheck,
		StatusCheckContexts:           form.StatusCheckContexts,
		EnableApprovalsWhitelist:      form.EnableApprovalsWhitelist,
		RequiredApprovals:             requiredApprovals,
		BlockOnRejectedReviews:        form.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: form.BlockOnOfficialReviewRequests,
		DismissStaleApprovals:         form.DismissStaleApprovals,
		IgnoreStaleApprovals:          form.IgnoreStaleApprovals,
		RequireSignedCommits:          form.RequireSignedCommits,
		ProtectedFilePatterns:         form.ProtectedFilePatterns,
		UnprotectedFilePatterns:       form.UnprotectedFilePatterns,
		BlockOnOutdatedBranch:         form.BlockOnOutdatedBranch,
		BlockAdminMergeOverride:       form.BlockAdminMergeOverride,
	}

	if err := git_model.UpdateProtectBranch(ctx, ctx.Repo.Repository, protectBranch, git_model.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		ForcePushUserIDs: forcePushAllowlistUsers,
		ForcePushTeamIDs: forcePushAllowlistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectBranch", err)
		return
	}

	if isBranchExist {
		if err := pull_service.CheckPRsForBaseBranch(ctx, ctx.Repo.Repository, ruleName); err != nil {
			ctx.Error(http.StatusInternalServerError, "CheckPRsForBaseBranch", err)
			return
		}
	} else {
		if !isPlainRule {
			if ctx.Repo.GitRepo == nil {
				ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
				if err != nil {
					ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
					return
				}
			}
			// FIXME: since we only need to recheck files protected rules, we could improve this
			matchedBranches, err := git_model.FindAllMatchedBranches(ctx, ctx.Repo.Repository.ID, ruleName)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "FindAllMatchedBranches", err)
				return
			}

			for _, branchName := range matchedBranches {
				if err = pull_service.CheckPRsForBaseBranch(ctx, ctx.Repo.Repository, branchName); err != nil {
					ctx.Error(http.StatusInternalServerError, "CheckPRsForBaseBranch", err)
					return
				}
			}
		}
	}

	// Reload from db to get all whitelists
	bp, err := git_model.GetProtectedBranchRuleByName(ctx, ctx.Repo.Repository.ID, ruleName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != ctx.Repo.Repository.ID {
		ctx.Error(http.StatusInternalServerError, "New branch protection not found", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToBranchProtection(ctx, bp, repo))
}

// EditBranchProtection edits a branch protection for a repo
func EditBranchProtection(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/branch_protections/{name} repository repoEditBranchProtection
	// ---
	// summary: Edit a branch protections for a repository. Only fields that are set will be changed
	// consumes:
	// - application/json
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
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditBranchProtectionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm(ctx).(*api.EditBranchProtectionOption)
	repo := ctx.Repo.Repository
	bpName := ctx.PathParam("name")
	protectBranch, err := git_model.GetProtectedBranchRuleByName(ctx, repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if protectBranch == nil || protectBranch.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	if form.EnablePush != nil {
		if !*form.EnablePush {
			protectBranch.CanPush = false
			protectBranch.EnableWhitelist = false
			protectBranch.WhitelistDeployKeys = false
		} else {
			protectBranch.CanPush = true
			if form.EnablePushWhitelist != nil {
				if !*form.EnablePushWhitelist {
					protectBranch.EnableWhitelist = false
					protectBranch.WhitelistDeployKeys = false
				} else {
					protectBranch.EnableWhitelist = true
					if form.PushWhitelistDeployKeys != nil {
						protectBranch.WhitelistDeployKeys = *form.PushWhitelistDeployKeys
					}
				}
			}
		}
	}

	if form.EnableForcePush != nil {
		if !*form.EnableForcePush {
			protectBranch.CanForcePush = false
			protectBranch.EnableForcePushAllowlist = false
			protectBranch.ForcePushAllowlistDeployKeys = false
		} else {
			protectBranch.CanForcePush = true
			if form.EnableForcePushAllowlist != nil {
				if !*form.EnableForcePushAllowlist {
					protectBranch.EnableForcePushAllowlist = false
					protectBranch.ForcePushAllowlistDeployKeys = false
				} else {
					protectBranch.EnableForcePushAllowlist = true
					if form.ForcePushAllowlistDeployKeys != nil {
						protectBranch.ForcePushAllowlistDeployKeys = *form.ForcePushAllowlistDeployKeys
					}
				}
			}
		}
	}

	if form.Priority != nil {
		protectBranch.Priority = *form.Priority
	}

	if form.EnableMergeWhitelist != nil {
		protectBranch.EnableMergeWhitelist = *form.EnableMergeWhitelist
	}

	if form.EnableStatusCheck != nil {
		protectBranch.EnableStatusCheck = *form.EnableStatusCheck
	}

	if form.StatusCheckContexts != nil {
		protectBranch.StatusCheckContexts = form.StatusCheckContexts
	}

	if form.RequiredApprovals != nil && *form.RequiredApprovals >= 0 {
		protectBranch.RequiredApprovals = *form.RequiredApprovals
	}

	if form.EnableApprovalsWhitelist != nil {
		protectBranch.EnableApprovalsWhitelist = *form.EnableApprovalsWhitelist
	}

	if form.BlockOnRejectedReviews != nil {
		protectBranch.BlockOnRejectedReviews = *form.BlockOnRejectedReviews
	}

	if form.BlockOnOfficialReviewRequests != nil {
		protectBranch.BlockOnOfficialReviewRequests = *form.BlockOnOfficialReviewRequests
	}

	if form.DismissStaleApprovals != nil {
		protectBranch.DismissStaleApprovals = *form.DismissStaleApprovals
	}

	if form.IgnoreStaleApprovals != nil {
		protectBranch.IgnoreStaleApprovals = *form.IgnoreStaleApprovals
	}

	if form.RequireSignedCommits != nil {
		protectBranch.RequireSignedCommits = *form.RequireSignedCommits
	}

	if form.ProtectedFilePatterns != nil {
		protectBranch.ProtectedFilePatterns = *form.ProtectedFilePatterns
	}

	if form.UnprotectedFilePatterns != nil {
		protectBranch.UnprotectedFilePatterns = *form.UnprotectedFilePatterns
	}

	if form.BlockOnOutdatedBranch != nil {
		protectBranch.BlockOnOutdatedBranch = *form.BlockOnOutdatedBranch
	}

	if form.BlockAdminMergeOverride != nil {
		protectBranch.BlockAdminMergeOverride = *form.BlockAdminMergeOverride
	}

	var whitelistUsers, forcePushAllowlistUsers, mergeWhitelistUsers, approvalsWhitelistUsers []int64
	if form.PushWhitelistUsernames != nil {
		whitelistUsers, err = user_model.GetUserIDsByNames(ctx, form.PushWhitelistUsernames, false)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		whitelistUsers = protectBranch.WhitelistUserIDs
	}
	if form.ForcePushAllowlistDeployKeys != nil {
		forcePushAllowlistUsers, err = user_model.GetUserIDsByNames(ctx, form.ForcePushAllowlistUsernames, false)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		forcePushAllowlistUsers = protectBranch.ForcePushAllowlistUserIDs
	}
	if form.MergeWhitelistUsernames != nil {
		mergeWhitelistUsers, err = user_model.GetUserIDsByNames(ctx, form.MergeWhitelistUsernames, false)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		mergeWhitelistUsers = protectBranch.MergeWhitelistUserIDs
	}
	if form.ApprovalsWhitelistUsernames != nil {
		approvalsWhitelistUsers, err = user_model.GetUserIDsByNames(ctx, form.ApprovalsWhitelistUsernames, false)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		approvalsWhitelistUsers = protectBranch.ApprovalsWhitelistUserIDs
	}

	var whitelistTeams, forcePushAllowlistTeams, mergeWhitelistTeams, approvalsWhitelistTeams []int64
	if repo.Owner.IsOrganization() {
		if form.PushWhitelistTeams != nil {
			whitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.PushWhitelistTeams, false)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			whitelistTeams = protectBranch.WhitelistTeamIDs
		}
		if form.ForcePushAllowlistTeams != nil {
			forcePushAllowlistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.ForcePushAllowlistTeams, false)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			forcePushAllowlistTeams = protectBranch.ForcePushAllowlistTeamIDs
		}
		if form.MergeWhitelistTeams != nil {
			mergeWhitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.MergeWhitelistTeams, false)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			mergeWhitelistTeams = protectBranch.MergeWhitelistTeamIDs
		}
		if form.ApprovalsWhitelistTeams != nil {
			approvalsWhitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.ApprovalsWhitelistTeams, false)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			approvalsWhitelistTeams = protectBranch.ApprovalsWhitelistTeamIDs
		}
	}

	err = git_model.UpdateProtectBranch(ctx, ctx.Repo.Repository, protectBranch, git_model.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		ForcePushUserIDs: forcePushAllowlistUsers,
		ForcePushTeamIDs: forcePushAllowlistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectBranch", err)
		return
	}

	isPlainRule := !git_model.IsRuleNameSpecial(bpName)
	var isBranchExist bool
	if isPlainRule {
		isBranchExist = git.IsBranchExist(ctx.Req.Context(), ctx.Repo.Repository.RepoPath(), bpName)
	}

	if isBranchExist {
		if err = pull_service.CheckPRsForBaseBranch(ctx, ctx.Repo.Repository, bpName); err != nil {
			ctx.Error(http.StatusInternalServerError, "CheckPrsForBaseBranch", err)
			return
		}
	} else {
		if !isPlainRule {
			if ctx.Repo.GitRepo == nil {
				ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
				if err != nil {
					ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
					return
				}
			}

			// FIXME: since we only need to recheck files protected rules, we could improve this
			matchedBranches, err := git_model.FindAllMatchedBranches(ctx, ctx.Repo.Repository.ID, protectBranch.RuleName)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "FindAllMatchedBranches", err)
				return
			}

			for _, branchName := range matchedBranches {
				if err = pull_service.CheckPRsForBaseBranch(ctx, ctx.Repo.Repository, branchName); err != nil {
					ctx.Error(http.StatusInternalServerError, "CheckPrsForBaseBranch", err)
					return
				}
			}
		}
	}

	// Reload from db to ensure get all whitelists
	bp, err := git_model.GetProtectedBranchRuleByName(ctx, repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchBy", err)
		return
	}
	if bp == nil || bp.RepoID != ctx.Repo.Repository.ID {
		ctx.Error(http.StatusInternalServerError, "New branch protection not found", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToBranchProtection(ctx, bp, repo))
}

// DeleteBranchProtection deletes a branch protection for a repo
func DeleteBranchProtection(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/branch_protections/{name} repository repoDeleteBranchProtection
	// ---
	// summary: Delete a specific branch protection for the repository
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
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	bpName := ctx.PathParam("name")
	bp, err := git_model.GetProtectedBranchRuleByName(ctx, repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	if err := git_model.DeleteProtectedBranch(ctx, ctx.Repo.Repository, bp.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProtectedBranch", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateBranchProtectionPriories updates the priorities of branch protections for a repo
func UpdateBranchProtectionPriories(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/branch_protections/priority repository repoUpdateBranchProtectionPriories
	// ---
	// summary: Update the priorities of branch protections for a repository.
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateBranchProtectionPriories"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm(ctx).(*api.UpdateBranchProtectionPriories)
	repo := ctx.Repo.Repository

	if err := git_model.UpdateProtectBranchPriorities(ctx, repo, form.IDs); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectBranchPriorities", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
