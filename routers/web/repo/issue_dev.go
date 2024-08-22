// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

func CreateBranchFromIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsPull {
		ctx.Flash.Error(ctx.Tr("repo.issues.create_branch_from_issue_error_is_pull"))
		ctx.JSONRedirect(issue.Link())
		return
	}

	form := web.GetForm(ctx).(*forms.NewBranchForm)
	repo := ctx.Repo.Repository
	gitRepo := ctx.Repo.GitRepo
	if form.RepoID > 0 {
		var err error
		repo, err = repo_model.GetRepositoryByID(ctx, form.RepoID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
		gitRepo, err = gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
		defer gitRepo.Close()
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	canCreateBranch := perm.CanWrite(unit_model.TypeCode) && repo.CanCreateBranch()
	if !canCreateBranch {
		ctx.NotFound("CreateBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	if err := repo_service.CreateNewBranch(ctx, ctx.Doer, repo, gitRepo, form.SourceBranchName, form.NewBranchName); err != nil {
		switch {
		case git_model.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err):
			ctx.JSONError(ctx.Tr("repo.branch.branch_already_exists", form.NewBranchName))
		case git_model.IsErrBranchNameConflict(err):
			e := err.(git_model.ErrBranchNameConflict)
			ctx.JSONError(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
		case git.IsErrPushRejected(err):
			e := err.(*git.ErrPushRejected)
			if len(e.Message) == 0 {
				ctx.Flash.Error(ctx.Tr("repo.editor.push_rejected_no_message"))
			} else {
				flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(e.Message),
				})
				if err != nil {
					ctx.ServerError("UpdatePullRequest.HTMLString", err)
					return
				}
				ctx.JSONError(flashError)
			}
		default:
			ctx.ServerError("CreateNewBranch", err)
		}
		return
	}

	if err := issues_model.CreateIssueDevLink(ctx, &issues_model.IssueDevLink{
		IssueID:      issue.ID,
		LinkType:     issues_model.IssueDevLinkTypeBranch,
		LinkedRepoID: repo.ID,
		LinkIndex:    form.NewBranchName,
	}); err != nil {
		ctx.ServerError("CreateIssueDevLink", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.issues.create_branch_from_issue_success", ctx.Repo.BranchName))
	ctx.JSONRedirect(issue.Link())
}
