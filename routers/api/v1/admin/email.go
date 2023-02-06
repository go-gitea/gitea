// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// GetAllEmails
func GetAllEmails(ctx *context.APIContext) {

	listOptions := utils.GetListOptions(ctx)

	emails, maxResults, err := user_model.SearchEmails(&user_model.SearchEmailOptions{
		Keyword:     ctx.Params(":email"),
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAllEmails", err)
		return
	}

	results := make([]*api.Email, len(emails))
	for i := range emails {
		results[i] = convert.ToEmailSearch(emails[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}

// GetEmail
func GetEmail(ctx *context.APIContext) {
	GetAllEmails(ctx)
}

// SearchEmail
func SearchEmail(ctx *context.APIContext) {
	ctx.SetParams(":email", ctx.FormTrim("q"))
	GetAllEmails(ctx)
}
