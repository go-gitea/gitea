// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"strings"

	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/util"
)

func ReqContextUserVisibleToDoer() func(ctx *Context) {
	return func(ctx *Context) {
		isOrg := ctx.ContextUser.IsOrganization()
		if isOrg && !organization.HasOrgOrUserVisible(ctx, ctx.ContextUser, ctx.Doer) ||
			!isOrg && !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
			ctx.NotFound()
			return
		}
	}
}

// UserAssignmentWeb returns a middleware to handle context-user assignment for web routes
func UserAssignmentWeb() func(ctx *Context) {
	return func(ctx *Context) {
		ctx.ContextUser = userAssignment(ctx.Base, ctx.Doer, ctx.ServerErrorAuto)
		ctx.Data["ContextUser"] = ctx.ContextUser
	}
}

// UserAssignmentAPI returns a middleware to handle context-user assignment for api routes
func UserAssignmentAPI() func(ctx *APIContext) {
	return func(ctx *APIContext) {
		ctx.ContextUser = userAssignment(ctx.Base, ctx.Doer, ctx.APIErrorAuto)
	}
}

func userAssignment(ctx *Base, doer *user_model.User, errCb func(error)) (contextUser *user_model.User) {
	username := ctx.PathParam("username")

	if doer != nil && strings.EqualFold(doer.LowerName, username) {
		contextUser = doer
	} else {
		var err error
		contextUser, err = user_model.GetUserByName(ctx, username)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				if redirectUserID, err := user_model.LookupUserRedirect(ctx, username); err == nil {
					RedirectToUser(ctx, doer, username, redirectUserID)
				} else if errors.Is(err, util.ErrNotExist) {
					errCb(util.NewNotExistErrorf("user not found"))
				} else {
					errCb(fmt.Errorf("LookupUserRedirect: %w", err))
				}
			} else {
				errCb(fmt.Errorf("GetUserByName: %w", err))
			}
			return nil
		}
	}
	if !user_model.DoerTokenAllowsOwner(doer, contextUser) {
		errCb(util.NewNotExistErrorf("user not found"))
		return nil
	}
	return contextUser
}

func IsDoerSessionImpersonated(sess session.Store) bool {
	return sess.Get(session.KeyImpersonatorData) != nil
}
