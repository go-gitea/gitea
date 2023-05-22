// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/convert"
)

// Search search users
func Search(ctx *context.Context) {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}

	users, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Keyword:     ctx.FormTrim("q"),
		UID:         ctx.FormInt64("uid"),
		Type:        user_model.UserTypeIndividual,
		IsActive:    ctx.FormOptionalBool("active"),
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	ctx.SetTotalCountHeader(maxResults)

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": convert.ToUsers(ctx, ctx.Doer, users),
	})
}
