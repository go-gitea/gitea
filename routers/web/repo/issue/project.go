// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
)

// UpdateIssueProject change an issue's project
func UpdateIssueProject(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	projectID := ctx.FormInt64("id")
	for _, issue := range issues {
		oldProjectID := issue.ProjectID()
		if oldProjectID == projectID {
			continue
		}

		if err := issues_model.ChangeProjectAssign(issue, ctx.Doer, projectID); err != nil {
			ctx.ServerError("ChangeProjectAssign", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}
