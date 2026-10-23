// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/log"
	"codenest.dev/modules/session"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/web/middleware"
	auth_service "codenest.dev/services/auth"
	"codenest.dev/services/context"
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
		ar.IsBasicAuth = ctx.Data["AuthedMethod"] == auth_service.BasicMethodName

		ctx.Data["IsSigned"] = true
		ctx.Data[middleware.ContextDataKeySignedUser] = ar.Doer
		ctx.Data["SignedUserID"] = ar.Doer.ID
		ctx.Data["IsAdmin"] = ar.Doer.IsAdmin

		if sessionStore != nil {
			if uid := auth_service.ImpersonatorUserID(sessionStore); uid != 0 {
				impersonator, err := user_model.GetUserByID(ctx, uid)
				if err != nil {
					// the session stays usable, but audit events must not silently lose the admin behind it
					log.Error("Unable to resolve impersonator %d: %v", uid, err)
				} else {
					ctx.Data[middleware.ContextDataKeyImpersonator] = impersonator
				}
			}
		}
	} else {
		ctx.Data["SignedUserID"] = int64(0)
	}
	return ar, nil
}

// VerifyOptions contains required or check options
type VerifyOptions struct {
	SignInRequired               bool
	SignOutRequired              bool
	AdminRequired                bool
	DisableCrossOriginProtection bool
}

func CheckSignedInUser(doer *user_model.User, sess session.Store) (ret struct {
	NeedActivateAccount bool
	LoginIsProhibited   bool
	NeedChangePassword  bool
},
) {
	ret.NeedActivateAccount = !doer.IsActive && setting.Service.RegisterEmailConfirm
	ret.LoginIsProhibited = !doer.IsActive || doer.ProhibitLogin
	isImpersonated := sess != nil && context.IsDoerSessionImpersonated(sess)
	ret.NeedChangePassword = doer.MustChangePassword && !isImpersonated && !doer.IsTypeBot()
	return ret
}
