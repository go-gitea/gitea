// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/web"
	"gitea.dev/routers/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	repo_service "gitea.dev/services/repository"
)

func CreateBranchFromIssue(ctx *context.Context) {
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsPull {
		ctx.Flash.Error(ctx.Tr("repo.issues.create_branch_from_issue_error_is_pull"))
		ctx.JSONRedirect(issue.Link())
		return
	}

	form := web.GetForm[*forms.NewBranchForm](ctx)
	repo := ctx.Repo.Repository
	if form.RepoID > 0 && form.RepoID != repo.ID {
		var err error
		repo, err = repo_model.GetRepositoryByID(ctx, form.RepoID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
	}

	perm, err := access_model.GetIndividualUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetIndividualUserRepoPermission", err)
		return
	}

	canCreateBranch := perm.CanWrite(unit_model.TypeCode) && repo.CanCreateBranch()
	if !canCreateBranch {
		ctx.HTTPError(http.StatusForbidden, "No permission to create branch in this repository")
		return
	}

	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		ctx.ServerError("RepositoryFromRequestContextOrOpen", err)
		return
	}

	if err := repo_service.CreateNewBranch(ctx, ctx.Doer, repo, gitRepo, form.SourceBranchName, form.NewBranchName); err != nil {
		switch {
		case git_model.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err):
			ctx.JSONError(ctx.Tr("repo.branch.branch_already_exists", form.NewBranchName))
		case git_model.IsErrBranchNameConflict(err):
			if e, ok := err.(git_model.ErrBranchNameConflict); ok {
				ctx.JSONError(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
			}
		case git_model.IsErrBranchNotExist(err):
			ctx.JSONError(ctx.Tr("repo.branch.branch_not_exist", form.SourceBranchName))
		case git.IsErrPushRejected(err):
			if e, ok := err.(*git.ErrPushRejected); ok {
				if len(e.Message) == 0 {
					ctx.Flash.Error(ctx.Tr("repo.editor.push_rejected_no_message"))
				} else {
					flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
						"Message": ctx.Tr("repo.editor.push_rejected"),
						"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
						"Details": utils.EscapeFlashErrorString(e.Message),
					})
					if err != nil {
						ctx.ServerError("UpdatePullRequest.HTMLString", err)
						return
					}
					ctx.JSONError(flashError)
				}
			}
		default:
			ctx.ServerError("CreateNewBranch", err)
		}
		return
	}

	branch, err := git_model.GetBranchExisting(ctx, repo.ID, form.NewBranchName)
	if err != nil {
		ctx.ServerError("GetBranch", err)
		return
	}

	if err := issues_model.CreateIssueDevLink(ctx, &issues_model.IssueDevLink{
		IssueID:      issue.ID,
		LinkType:     issues_model.IssueDevLinkTypeBranch,
		LinkedRepoID: repo.ID,
		LinkID:       branch.ID,
	}); err != nil {
		ctx.ServerError("CreateIssueDevLink", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.issues.create_branch_from_issue_success", form.NewBranchName))
	ctx.JSONRedirect(issue.Link())
}
