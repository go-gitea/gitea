// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {

	depID := ctx.QueryInt64("newDependency")

	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(issue, ctx.User) {
		ctx.Handle(404, "MustEnableIssueDependencies", nil)
		return
	}

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
		url := fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex)
		ctx.Redirect(url, http.StatusSeeOther)
		return
	}

	// Check if both issues are in the same repo
	if issue.RepoID != dep.RepoID {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
		url := fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex)
		ctx.Redirect(url, http.StatusSeeOther)
		return
	}

	// Check if issue and dependency is the same
	if dep.Index == issueIndex {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
	} else {
		err := models.CreateIssueDependency(ctx.User, issue, dep)
		if err != nil {
			if models.IsErrDependencyExists(err) {
				ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
			} else if models.IsErrCircularDependency(err) {
				ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
			} else {
				ctx.Handle(http.StatusInternalServerError, "CreateOrUpdateIssueDependency", err)
				return
			}
		}
	}

	url := fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex)
	ctx.Redirect(url, http.StatusSeeOther)
}
