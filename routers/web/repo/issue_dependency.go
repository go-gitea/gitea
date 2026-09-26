// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/modules/setting"
	"gitea.dev/services/context"
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
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
		return
	}

	depID := ctx.FormInt64("newDependency")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
		return
	}

	// Check if both issues are in the same repo if cross repository dependencies is not enabled
	if issue.RepoID != dep.RepoID {
		if !setting.Service.AllowCrossRepositoryDependencies {
			ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
			return
		}
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerError("loadRepo", err)
			return
		}
		// Can ctx.Doer read issues in the dep repo?
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetDoerRepoPermission", err)
			return
		}
		if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
			ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
			return
		}
	}

	// Check if issue and dependency is the same
	if dep.ID == issue.ID {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, dep)
	if issues_model.IsErrDependencyExists(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
		return
	} else if issues_model.IsErrCircularDependency(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
		return
	} else if err != nil {
		ctx.ServerError("CreateOrUpdateIssueDependency", err)
		return
	}
	ctx.JSONOK()
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
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
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
		ctx.JSONError("invalid dependency type")
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	// Existing cross-repo dependencies must remain removable even when
	// AllowCrossRepositoryDependencies is disabled, so only enforce that the
	// doer can read the dependency's repository.
	if issue.RepoID != dep.RepoID {
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerError("loadRepo", err)
			return
		}
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetDoerRepoPermission", err)
			return
		}
		if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
			ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
			return
		}
	}

	err = issues_model.RemoveIssueDependency(ctx, ctx.Doer, issue, dep, depType)
	if issues_model.IsErrDependencyNotExists(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
		return
	} else if err != nil {
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}
	ctx.JSONOK()
}
