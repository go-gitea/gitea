// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// RemoveDependency removes the dependency
func RemoveDependency(c *context.Context) {
	depID, err := strconv.ParseInt(c.Req.PostForm.Get("removeDependencyID"), 10, 64)
	if err != nil {
		c.Handle(http.StatusBadRequest, "issue ID is not int", err)
		return
	}

	issueIndex := c.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(c.Repo.Repository.ID, issueIndex)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !c.Repo.CanCreateIssueDependencies(issue, c.User) {
		c.Handle(404, "MustEnableIssueDependencies", nil)
		return
	}

	// Dependency Type
	depTypeStr := c.Req.PostForm.Get("dependencyType")

	var depType models.DependencyType

	switch depTypeStr {
	case "blockedBy":
		depType = models.DependencyTypeBlockedBy
	case "blocking":
		depType = models.DependencyTypeBlocking
	default:
		c.Handle(http.StatusBadRequest, "GetDependecyType", nil)
		return
	}

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByID", err)
		return
	}

	if err = models.RemoveIssueDependency(c.User, issue, dep, depType); err != nil {
		c.Handle(http.StatusInternalServerError, "CreateOrUpdateIssueDependency", err)
		return
	}

	url := fmt.Sprintf("%s/issues/%d", c.Repo.RepoLink, issueIndex)
	c.Redirect(url, http.StatusSeeOther)
}
