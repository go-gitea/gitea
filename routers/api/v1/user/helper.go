// Copyright 2021 The Gitea Authors.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
)

// GetUserByPathParam get user by the path param name
// it will redirect to the user's new name if the user's name has been changed
func GetUserByPathParam(ctx *context.APIContext, name string) *user_model.User {
	username := ctx.PathParam(name)
	user, err := user_model.GetUserByName(ctx, username)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			if redirectUserID, err2 := user_model.LookupUserRedirect(ctx, username); err2 == nil {
				context.RedirectToUser(ctx.Base, username, redirectUserID)
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

// GetContextUserByPathParam returns user whose name is presented in URL (path param "username").
func GetContextUserByPathParam(ctx *context.APIContext) *user_model.User {
	return GetUserByPathParam(ctx, "username")
}
