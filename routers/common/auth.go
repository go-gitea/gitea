// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"
	auth_service "code.gitea.io/gitea/services/auth"
)

type AuthResult struct {
	Doer        *user_model.User
	IsBasicAuth bool
}

func AuthShared(ctx *context.Base, sessionStore auth_service.SessionStore, authMethod auth_service.Method) (ar AuthResult, err error) {
	ar.Doer, err = authMethod.Verify(ctx.Req, ctx.Resp, ctx, sessionStore)
	if err != nil {
		return ar, err
	}
	if ar.Doer != nil {
		if ctx.Locale.Language() != ar.Doer.Language {
			ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
		}
		ar.IsBasicAuth = ctx.Data["AuthedMethod"].(string) == auth_service.BasicMethodName

		ctx.Data["IsSigned"] = true
		ctx.Data[middleware.ContextDataKeySignedUser] = ar.Doer
		ctx.Data["SignedUserID"] = ar.Doer.ID
		ctx.Data["IsAdmin"] = ar.Doer.IsAdmin
	} else {
		ctx.Data["SignedUserID"] = int64(0)
	}
	return ar, nil
}

// VerifyOptions contains required or check options
type VerifyOptions struct {
	SignInRequired  bool
	SignOutRequired bool
	AdminRequired   bool
	DisableCSRF     bool
}
