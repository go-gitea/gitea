// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.FormInt64("newDependency")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Redirect
	defer ctx.Redirect(issue.Link())

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
		return
	}

	// Check if both issues are in the same repo if cross repository dependencies is not enabled
	if issue.RepoID != dep.RepoID {
		if !setting.Service.AllowCrossRepositoryDependencies {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
			return
		}
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerError("loadRepo", err)
			return
		}
		// Can ctx.Doer read issues in the dep repo?
		depRepoPerm, err := access_model.GetUserRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return
		}
		if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
			// you can't see this dependency
			return
		}
	}

	// Check if issue and dependency is the same
	if dep.ID == issue.ID {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, dep)
	if err != nil {
		if issues_model.IsErrDependencyExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
			return
		} else if issues_model.IsErrCircularDependency(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
			return
		}
		ctx.ServerError("CreateOrUpdateIssueDependency", err)
		return
	}
}

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.FormInt64("removeDependencyID")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Dependency Type
	depTypeStr := ctx.Req.PostFormValue("dependencyType")

	var depType issues_model.DependencyType

	switch depTypeStr {
	case "blockedBy":
		depType = issues_model.DependencyTypeBlockedBy
	case "blocking":
		depType = issues_model.DependencyTypeBlocking
	default:
		ctx.HTTPError(http.StatusBadRequest, "GetDependecyType")
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = issues_model.RemoveIssueDependency(ctx, ctx.Doer, issue, dep, depType); err != nil {
		if issues_model.IsErrDependencyNotExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
			return
		}
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}

	// Redirect
	ctx.Redirect(issue.Link())
}
