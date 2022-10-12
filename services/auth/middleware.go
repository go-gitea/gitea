// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"
)

// Auth is a middleware to authenticate a web user
func Auth(authMethod Method) func(*context.Context) {
	return func(ctx *context.Context) {
		authShared(ctx, authMethod)
		if ctx.Doer == nil {
			// ensure the session uid is deleted
			_ = ctx.Session.Delete("uid")
		}
	}
}

// APIAuth is a middleware to authenticate an api user
func APIAuth(authMethod Method) func(*context.APIContext) {
	return func(ctx *context.APIContext) {
		authShared(ctx.Context, authMethod)
	}
}

func authShared(ctx *context.Context, authMethod Method) {
	ctx.Doer = authMethod.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
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
}