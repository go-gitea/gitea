// Copyright 2021 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
)

// GetUserByParamsName get user by name
func GetUserByParamsName(ctx *context.APIContext, name string) *models.User {
	username := ctx.Params(name)
	user, err := models.GetUserByName(username)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			if redirectUserID, err2 := user_model.LookupUserRedirect(username); err2 == nil {
				context.RedirectToUser(ctx.Context, username, redirectUserID)
			} else {
				ctx.NotFound("GetUserByName", err)
			}
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL (":username").
func GetUserByParams(ctx *context.APIContext) *models.User {
	return GetUserByParamsName(ctx, ":username")
}
