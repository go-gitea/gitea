// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/externalaccount"

	"github.com/tstranex/u2f"
)

var tplU2F base.TplName = "user/auth/u2f"

// U2F shows the U2F login page
func U2F(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa")
	ctx.Data["RequireU2F"] = true
	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	// Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}

	// See whether TOTP is also available.
	if ctx.Session.Get("totpEnrolled") != nil {
		ctx.Data["TOTPEnrolled"] = true
	}

	ctx.HTML(http.StatusOK, tplU2F)
}

// U2FChallenge submits a sign challenge to the browser
func U2FChallenge(ctx *context.Context) {
	// Ensure user is in a U2F session.
	idSess := ctx.Session.Get("twofaUid")
	if idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}
	id := idSess.(int64)
	regs, err := auth.GetU2FRegistrationsByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	if len(regs) == 0 {
		ctx.ServerError("UserSignIn", errors.New("no device registered"))
		return
	}
	challenge, err := u2f.NewChallenge(setting.U2F.AppID, setting.U2F.TrustedFacets)
	if err != nil {
		ctx.ServerError("u2f.NewChallenge", err)
		return
	}
	if err := ctx.Session.Set("u2fChallenge", challenge); err != nil {
		ctx.ServerError("UserSignIn: unable to set u2fChallenge in session", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("UserSignIn: unable to store session", err)
	}

	ctx.JSON(http.StatusOK, challenge.SignRequest(regs.ToRegistrations()))
}

// U2FSign authenticates the user by signResp
func U2FSign(ctx *context.Context) {
	signResp := web.GetForm(ctx).(*u2f.SignResponse)
	challSess := ctx.Session.Get("u2fChallenge")
	idSess := ctx.Session.Get("twofaUid")
	if challSess == nil || idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}
	challenge := challSess.(*u2f.Challenge)
	id := idSess.(int64)
	regs, err := auth.GetU2FRegistrationsByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	for _, reg := range regs {
		r, err := reg.Parse()
		if err != nil {
			log.Error("parsing u2f registration: %v", err)
			continue
		}
		newCounter, authErr := r.Authenticate(*signResp, *challenge, reg.Counter)
		if authErr == nil {
			reg.Counter = newCounter
			user, err := user_model.GetUserByID(id)
			if err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}
			remember := ctx.Session.Get("twofaRemember").(bool)
			if err := reg.UpdateCounter(); err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}

			if ctx.Session.Get("linkAccount") != nil {
				if err := externalaccount.LinkAccountFromStore(ctx.Session, user); err != nil {
					ctx.ServerError("UserSignIn", err)
					return
				}
			}
			redirect := handleSignInFull(ctx, user, remember, false)
			if ctx.Written() {
				return
			}
			if redirect == "" {
				redirect = setting.AppSubURL + "/"
			}
			ctx.PlainText(http.StatusOK, redirect)
			return
		}
	}
	ctx.Error(http.StatusUnauthorized)
}
