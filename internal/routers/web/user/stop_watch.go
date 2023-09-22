// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"code.gitea.io/gitea/internal/models/db"
	issues_model "code.gitea.io/gitea/internal/models/issues"
	"code.gitea.io/gitea/internal/modules/context"
	"code.gitea.io/gitea/internal/services/convert"
)

// GetStopwatches get all stopwatches
func GetStopwatches(ctx *context.Context) {
	sws, err := issues_model.GetUserStopwatches(ctx, ctx.Doer.ID, db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	count, err := issues_model.CountUserStopwatches(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	apiSWs, err := convert.ToStopWatches(sws)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSWs)
}
