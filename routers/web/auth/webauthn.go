// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/binary"
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	wa "code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/externalaccount"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var tplWebAuthn templates.TplName = "user/auth/webauthn"

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

// WebAuthnPasskeyAssertion submits a WebAuthn challenge for the passkey login to the browser
func WebAuthnPasskeyAssertion(ctx *context.Context) {
	assertion, sessionData, err := wa.WebAuthn.BeginDiscoverableLogin()
	if err != nil {
		ctx.ServerError("webauthn.BeginDiscoverableLogin", err)
		return
	}

	if err := ctx.Session.Set("webauthnPasskeyAssertion", sessionData); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}

	ctx.JSON(http.StatusOK, assertion)
}

// WebAuthnPasskeyLogin handles the WebAuthn login process using a Passkey
func WebAuthnPasskeyLogin(ctx *context.Context) {
	sessionData, okData := ctx.Session.Get("webauthnPasskeyAssertion").(*webauthn.SessionData)
	if !okData || sessionData == nil {
		ctx.ServerError("ctx.Session.Get", errors.New("not in WebAuthn session"))
		return
	}
	defer func() {
		_ = ctx.Session.Delete("webauthnPasskeyAssertion")
	}()

	// Validate the parsed response.

	// ParseCredentialRequestResponse+ValidateDiscoverableLogin equals to FinishDiscoverableLogin, but we need to ParseCredentialRequestResponse first to get flags
	var user *user_model.User
	parsedResponse, err := protocol.ParseCredentialRequestResponse(ctx.Req)
	if err != nil {
		// Failed authentication attempt.
		log.Info("Failed authentication attempt for %s from %s: %v", user.Name, ctx.RemoteAddr(), err)
		ctx.Status(http.StatusForbidden)
		return
	}
	cred, err := wa.WebAuthn.ValidateDiscoverableLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		userID, n := binary.Varint(userHandle)
		if n <= 0 {
			return nil, errors.New("invalid rawID")
		}

		var err error
		user, err = user_model.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}

		return wa.NewWebAuthnUser(ctx, user, parsedResponse.Response.AuthenticatorData.Flags), nil
	}, *sessionData, parsedResponse)
	if err != nil {
		// Failed authentication attempt.
		log.Info("Failed authentication attempt for passkey from %s: %v", ctx.RemoteAddr(), err)
		ctx.Status(http.StatusForbidden)
		return
	}

	if !cred.Flags.UserPresent {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if user == nil {
		ctx.Status(http.StatusBadRequest)
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

	remember := false // TODO: implement remember me
	redirect := handleSignInFull(ctx, user, remember, false)
	if redirect == "" {
		redirect = setting.AppSubURL + "/"
	}

	ctx.JSONRedirect(redirect)
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

	webAuthnUser := wa.NewWebAuthnUser(ctx, user)
	assertion, sessionData, err := wa.WebAuthn.BeginLogin(webAuthnUser)
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
	webAuthnUser := wa.NewWebAuthnUser(ctx, user, parsedResponse.Response.AuthenticatorData.Flags)
	cred, err := wa.WebAuthn.ValidateLogin(webAuthnUser, *sessionData, parsedResponse)
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
