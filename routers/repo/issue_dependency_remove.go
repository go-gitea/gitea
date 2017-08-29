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

// IssueWatch sets issue watching
func RemoveDependency(c *context.Context) {
	dep, err := strconv.ParseInt(c.Req.PostForm.Get("removeDependencyID"), 10, 64)
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

	// Dependency Type
	// Types: 1 = blockedBy, 2 = blocking
	depType, err := strconv.ParseInt(c.Req.PostForm.Get("dependencyType"), 10, 64)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetDependecyType", err)
		return
	}

	if depType != 1 && depType != 2{
		c.Handle(http.StatusBadRequest, "GetDependecyType", nil)
		return
	}

	err = models.RemoveIssueDependency(c.User.ID, issue.ID, dep, depType)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "CreateOrUpdateIssueDependency", err)
		return
	}

	url := fmt.Sprintf("%s/issues/%d", c.Repo.RepoLink, issueIndex)
	c.Redirect(url, http.StatusSeeOther)
}
