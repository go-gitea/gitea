// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
)

// UserAssignmentWeb returns a middleware to handle context-user assignment for web routes
func UserAssignmentWeb() func(ctx *context.Context) {
	return func(ctx *context.Context) {
		errorFn := func(status int, title string, obj any) {
			err, ok := obj.(error)
			if !ok {
				err = fmt.Errorf("%s", obj)
			}
			if status == http.StatusNotFound {
				ctx.NotFound(title, err)
			} else {
				ctx.ServerError(title, err)
			}
		}
		ctx.ContextUser = userAssignment(ctx.Base, ctx.Doer, errorFn)
		ctx.Data["ContextUser"] = ctx.ContextUser
	}
}

// UserIDAssignmentAPI returns a middleware to handle context-user assignment for api routes
func UserIDAssignmentAPI() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		userID := ctx.ParamsInt64(":user-id")

		if ctx.IsSigned && ctx.Doer.ID == userID {
			ctx.ContextUser = ctx.Doer
		} else {
			var err error
			ctx.ContextUser, err = user_model.GetUserByID(ctx, userID)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.Error(http.StatusNotFound, "GetUserByID", err)
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByID", err)
				}
			}
		}
	}
}

// UserAssignmentAPI returns a middleware to handle context-user assignment for api routes
func UserAssignmentAPI() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		ctx.ContextUser = userAssignment(ctx.Base, ctx.Doer, ctx.Error)
	}
}

func userAssignment(ctx *context.Base, doer *user_model.User, errCb func(int, string, any)) (contextUser *user_model.User) {
	username := ctx.Params(":username")

	if doer != nil && doer.LowerName == strings.ToLower(username) {
		contextUser = doer
	} else {
		var err error
		contextUser, err = user_model.GetUserByName(ctx, username)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				if redirectUserID, err := user_model.LookupUserRedirect(ctx, username); err == nil {
					context.RedirectToUser(ctx, username, redirectUserID)
				} else if user_model.IsErrUserRedirectNotExist(err) {
					errCb(http.StatusNotFound, "GetUserByName", err)
				} else {
					errCb(http.StatusInternalServerError, "LookupUserRedirect", err)
				}
			} else {
				errCb(http.StatusInternalServerError, "GetUserByName", err)
			}
		}
	}
	return contextUser
}
