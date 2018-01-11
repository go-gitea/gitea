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

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	depID := ctx.QueryInt64("removeDependencyID")

	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(issue, ctx.User) {
		ctx.NotFound("NotAllowedToCreateIssueDependencies", nil)
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
		//ctx.Handle(http.StatusBadRequest, "GetDependecyType", nil)
		return
	}

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = models.RemoveIssueDependency(ctx.User, issue, dep, depType); err != nil {
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}

	url := fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex)
	ctx.Redirect(url, http.StatusSeeOther)
}
