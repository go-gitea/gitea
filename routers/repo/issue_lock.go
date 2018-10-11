// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo.
func LockIssue(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}

	if err := models.LockIssue(c.User, issue); err != nil {
		c.ServerError("LockIssue", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}
