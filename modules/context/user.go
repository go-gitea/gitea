// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
)

// UserAssignment returns a middleware to handle context-user assignment
func UserAssignment() func(ctx *Context) {
	return func(ctx *Context) {
		userAssignment(ctx, func(status int, title string, obj interface{}) {
			err, ok := obj.(error)
			if !ok {
				err = fmt.Errorf("%s", obj)
			}
			if status == http.StatusNotFound {
				ctx.NotFound(title, err)
			} else {
				ctx.ServerError(title, err)
			}
		})
	}
}

// UserAssignmentAPI returns a middleware to handle context-user assignment
func UserAssignmentAPI() func(ctx *APIContext) {
	return func(ctx *APIContext) {
		userAssignment(ctx.Context, ctx.Error)
	}
}

func userAssignment(ctx *Context, errCb func(int, string, interface{})) {
	username := ctx.Params(":username")

	if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(username) {
		ctx.ContextUser = ctx.User
	} else {
		var err error
		ctx.ContextUser, err = user_model.GetUserByName(username)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				if redirectUserID, err := user_model.LookupUserRedirect(username); err == nil {
					RedirectToUser(ctx, username, redirectUserID)
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
}
