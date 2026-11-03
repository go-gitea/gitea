// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	issue_service "gitea.dev/services/issue"
)

// UpdateIssueConfidential makes an issue confidential or public.
func UpdateIssueConfidential(ctx *context.Context) {
	form := web.GetForm[*forms.IssueConfidentialForm](ctx)
	issue := GetActionIssue(ctx)
	if issue == nil {
		return
	}
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	err := issue_service.ChangeConfidential(ctx, issue, ctx.Doer, ctx.Repo.Permission, form.Confidential == "true")
	switch {
	case errors.Is(err, issue_service.ErrConfidentialPermissionDenied):
		ctx.JSON(http.StatusForbidden, map[string]any{"errorMessage": ctx.Locale.TrString("repo.issues.confidential.no_permission"), "renderFormat": "text"})
	case errors.Is(err, issue_service.ErrConfidentialPullRequest):
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{"errorMessage": ctx.Locale.TrString("repo.issues.confidential.pull_request"), "renderFormat": "text"})
	case err != nil:
		ctx.ServerError("ChangeConfidential", err)
	default:
		ctx.JSONRedirect(issue.Link())
	}
}
