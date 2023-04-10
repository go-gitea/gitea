// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web/middleware"
)

// Auth is a middleware to authenticate a web user
func Auth(authMethod Method) func(*context.Context) {
	return func(ctx *context.Context) {
		if err := authShared(ctx, authMethod); err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.Error(http.StatusUnauthorized, "Verify")
			return
		}
		if ctx.Doer == nil {
			// ensure the session uid is deleted
			_ = ctx.Session.Delete("uid")
		}
	}
}

// APIAuth is a middleware to authenticate an api user
func APIAuth(authMethod Method) func(*context.APIContext) {
	return func(ctx *context.APIContext) {
		if err := authShared(ctx.Context, authMethod); err != nil {
			ctx.Error(http.StatusUnauthorized, "APIAuth", err)
		}
	}
}

func authShared(ctx *context.Context, authMethod Method) error {
	var err error
	ctx.Doer, err = authMethod.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
	if err != nil {
		return err
	}
	if ctx.Doer != nil {
		if ctx.Locale.Language() != ctx.Doer.Language {
			ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
		}
		ctx.IsBasicAuth = ctx.Data["AuthedMethod"].(string) == BasicMethodName
		ctx.IsSigned = true
		ctx.Data["IsSigned"] = ctx.IsSigned
		ctx.Data["SignedUser"] = ctx.Doer
		ctx.Data["SignedUserID"] = ctx.Doer.ID
		ctx.Data["SignedUserName"] = ctx.Doer.Name
		ctx.Data["IsAdmin"] = ctx.Doer.IsAdmin
	} else {
		ctx.Data["SignedUserID"] = int64(0)
		ctx.Data["SignedUserName"] = ""
	}
	return nil
}
