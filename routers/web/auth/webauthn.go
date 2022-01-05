// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"encoding/base64"
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

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"
)

var tplWebAuthn base.TplName = "user/auth/webauthn"

// WebAuthn shows the WebAuthn login page
func WebAuthn(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa")

	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	//Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}

	ctx.HTML(200, tplWebAuthn)
}

// WebAuthnLoginAssertion submits a WebAuthn challenge to the browser
func WebAuthnLoginAssertion(ctx *context.Context) {
	// Ensure user is in a WebAuthn session.
	idSess := ctx.Session.Get("twofaUid")
	if idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}

	user, err := user_model.GetUserByID(idSess.(int64))
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	creds, err := auth.WebAuthnCredentials(user.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	if len(creds) == 0 {
		ctx.ServerError("UserSignIn", errors.New("no device registered"))
		return
	}

	userVerification := ctx.FormString("user_ver")
	txAuthSimpleExtension := ctx.FormString("tx_auth_simple_extension")
	testExtension := protocol.AuthenticationExtensions(map[string]interface{}{"txAuthSimple": txAuthSimpleExtension})

	assertion, sessionData, err := wa.WebAuthn.BeginLogin(
		(*wa.User)(user),
		webauthn.WithUserVerification(protocol.UserVerificationRequirement(userVerification)),
		webauthn.WithAssertionExtensions(testExtension),
	)
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
	idSess := ctx.Session.Get("twofaUid")
	sessionData := ctx.Session.Get("webauthnAssertion")
	if sessionData == nil || idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in WebAuthn session"))
		return
	}
	defer func() {
		_ = ctx.Session.Delete("webauthnAssertion")
	}()
	user, err := user_model.GetUserByID(idSess.(int64))
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	log.Trace("Finishing webauthn authentication with user: %s\n", user.Name)

	// With the session data retrieved, we need to call webauthn.FinishLogin to
	// verify the signed challenge. This returns the webauthn.Credential that
	// was used to authenticate.
	cred, err := wa.WebAuthn.FinishLogin((*wa.User)(user), *sessionData.(*webauthn.SessionData), ctx.Req)
	if err != nil {
		ctx.ServerError("FinishLogin", err)
		return
	}

	// At this point, we've confirmed the correct authenticator has been
	// provided and it passed the challenge we gave it. We now need to make
	// sure that the sign counter is higher than what we have stored to help
	// give assurance that this credential wasn't cloned.
	if cred.Authenticator.CloneWarning {
		log.Error("credential appears to be cloned: %s", err)
		ctx.Status(http.StatusForbidden)
		return
	}
	// We're logged in! All that's left is to update the sign count with the
	// new value we received. We could join the tables on the CredentialID
	// field, but for our purposes we'll just get the stored credential and
	// use that to find the authenticator we need to update.
	dbCred, err := auth.GetWebAuthnCredentialByCredID(base64.RawStdEncoding.EncodeToString(cred.ID))
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialByCredID", err)
		return
	}

	dbCred.SignCount = cred.Authenticator.SignCount
	if err := dbCred.UpdateSignCount(); err != nil {
		ctx.ServerError("UpdateSignCount", err)
		return
	}

	if ctx.Session.Get("linkAccount") != nil {
		if err := externalaccount.LinkAccountFromStore(ctx.Session, user); err != nil {
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
	ctx.JSON(200, map[string]string{"redirect": redirect})
}
