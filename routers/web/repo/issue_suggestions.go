// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	issue_service "gitea.dev/services/issue"
)

// IssueSuggestions returns a list of issue suggestions
func IssueSuggestions(ctx *context.Context) {
	keyword := ctx.Req.FormValue("q")

	isPull, ok := common.ParseIssueFilterTypeIsPullWithPermission(ctx.Repo.Permission, "")
	if !ok {
		ctx.HTTPError(http.StatusForbidden, "no permission to read issues or pulls")
		return
	}

	suggestions, err := issue_service.GetSuggestion(ctx, ctx.Repo.Repository, isPull, keyword)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.JSON(http.StatusOK, suggestions)
}
