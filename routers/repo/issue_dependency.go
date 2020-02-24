// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {
	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx.User, issue.IsPull) {
		ctx.Error(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.QueryInt64("newDependency")

	if err = issue.LoadRepo(); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Redirect
	defer ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
		return
	}

	// Check if both issues are in the same repo if cross repository dependencies is not enabled
	if issue.RepoID != dep.RepoID && !setting.Service.AllowCrossRepositoryDependencies {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
		return
	}

	// Check if issue and dependency is the same
	if dep.ID == issue.ID {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = models.CreateIssueDependency(ctx.User, issue, dep)
	if err != nil {
		if models.IsErrDependencyExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
			return
		} else if models.IsErrCircularDependency(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
			return
		} else {
			ctx.ServerError("CreateOrUpdateIssueDependency", err)
			return
		}
	}
}

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx.User, issue.IsPull) {
		ctx.Error(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.QueryInt64("removeDependencyID")

	if err = issue.LoadRepo(); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Dependency Type
	depTypeStr := ctx.Req.PostForm.Get("dependencyType")

	var depType models.DependencyType

	switch depTypeStr {
	case "blockedBy":
		depType = models.DependencyTypeBlockedBy
	case "blocking":
		depType = models.DependencyTypeBlocking
	default:
		ctx.Error(http.StatusBadRequest, "GetDependecyType")
		return
	}

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = models.RemoveIssueDependency(ctx.User, issue, dep, depType); err != nil {
		if models.IsErrDependencyNotExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
			return
		}
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}

	// Redirect
	ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)
}
