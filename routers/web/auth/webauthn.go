// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	wa "code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/externalaccount"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var tplWebAuthn base.TplName = "user/auth/webauthn"

// WebAuthn shows the WebAuthn login page
func WebAuthn(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa")

	if CheckAutoLogin(ctx) {
		return
	}

	// Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}

	hasTwoFactor, err := auth.HasTwoFactorByUID(ctx, ctx.Session.Get("twofaUid").(int64))
	if err != nil {
		ctx.ServerError("HasTwoFactorByUID", err)
		return
	}

	ctx.Data["HasTwoFactor"] = hasTwoFactor

	ctx.HTML(http.StatusOK, tplWebAuthn)
}

// WebAuthnLoginAssertion submits a WebAuthn challenge to the browser
func WebAuthnLoginAssertion(ctx *context.Context) {
	// Ensure user is in a WebAuthn session.
	idSess, ok := ctx.Session.Get("twofaUid").(int64)
	if !ok || idSess == 0 {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}

	user, err := user_model.GetUserByID(ctx, idSess)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	exists, err := auth.ExistsWebAuthnCredentialsForUID(ctx, user.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	if !exists {
		ctx.ServerError("UserSignIn", errors.New("no device registered"))
		return
	}

	assertion, sessionData, err := wa.WebAuthn.BeginLogin((*wa.User)(user))
	if err != nil {
		ctx.ServerError("webauthn.BeginLogin", err)
		return
	}

	if err := ctx.Session.Set("webauthnAssertion", sessionData); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	ctx.JSON(http.StatusOK, assertion)
}

// WebAuthnLoginAssertionPost validates the signature and logs the user in
func WebAuthnLoginAssertionPost(ctx *context.Context) {
	idSess, ok := ctx.Session.Get("twofaUid").(int64)
	sessionData, okData := ctx.Session.Get("webauthnAssertion").(*webauthn.SessionData)
	if !ok || !okData || sessionData == nil || idSess == 0 {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}
	defer func() {
		_ = ctx.Session.Delete("webauthnAssertion")
	}()

	// Load the user from the db
	user, err := user_model.GetUserByID(ctx, idSess)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	log.Trace("Finishing webauthn authentication with user: %s", user.Name)

	// Now we do the equivalent of webauthn.FinishLogin using a combination of our session data
	// (from webauthnAssertion) and verify the provided request.0
	parsedResponse, err := protocol.ParseCredentialRequestResponse(ctx.Req)
	if err != nil {
		// Failed authentication attempt.
		log.Info("Failed authentication attempt for %s from %s: %v", user.Name, ctx.RemoteAddr(), err)
		ctx.Status(http.StatusForbidden)
		return
	}

	// Validate the parsed response.
	cred, err := wa.WebAuthn.ValidateLogin((*wa.User)(user), *sessionData, parsedResponse)
	if err != nil {
		// Failed authentication attempt.
		log.Info("Failed authentication attempt for %s from %s: %v", user.Name, ctx.RemoteAddr(), err)
		ctx.Status(http.StatusForbidden)
		return
	}

	// Ensure that the credential wasn't cloned by checking if CloneWarning is set.
	// (This is set if the sign counter is less than the one we have stored.)
	if cred.Authenticator.CloneWarning {
		log.Info("Failed authentication attempt for %s from %s: cloned credential", user.Name, ctx.RemoteAddr())
		ctx.Status(http.StatusForbidden)
		return
	}

	// Success! Get the credential and update the sign count with the new value we received.
	dbCred, err := auth.GetWebAuthnCredentialByCredID(ctx, user.ID, cred.ID)
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialByCredID", err)
		return
	}

	dbCred.SignCount = cred.Authenticator.SignCount
	if err := dbCred.UpdateSignCount(ctx); err != nil {
		ctx.ServerError("UpdateSignCount", err)
		return
	}

	// Now handle account linking if that's requested
	if ctx.Session.Get("linkAccount") != nil {
		if err := externalaccount.LinkAccountFromStore(ctx, ctx.Session, user); err != nil {
			ctx.ServerError("LinkAccountFromStore", err)
			return
		}
	}

	remember := ctx.Session.Get("twofaRemember").(bool)
	redirect := handleSignInFull(ctx, user, remember, false)
	if redirect == "" {
		redirect = setting.AppSubURL + "/"
	}
	_ = ctx.Session.Delete("twofaUid")

	ctx.JSONRedirect(redirect)
}
