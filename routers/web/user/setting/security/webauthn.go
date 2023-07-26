// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/models/auth"
	wa "code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnRegister initializes the webauthn registration procedure
func WebAuthnRegister(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.WebauthnRegistrationForm)
	if form.Name == "" {
		// Set name to the hexadecimal of the current time
		form.Name = strconv.FormatInt(time.Now().UnixNano(), 16)
	}

	cred, err := auth.GetWebAuthnCredentialByName(ctx.Doer.ID, form.Name)
	if err != nil && !auth.IsErrWebAuthnCredentialNotExist(err) {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	if cred != nil {
		ctx.Error(http.StatusConflict, "Name already taken")
		return
	}

	_ = ctx.Session.Delete("webauthnRegistration")
	if err := ctx.Session.Set("webauthnName", form.Name); err != nil {
		ctx.ServerError("Unable to set session key for webauthnName", err)
		return
	}

	credentialOptions, sessionData, err := wa.WebAuthn.BeginRegistration((*wa.User)(ctx.Doer))
	if err != nil {
		ctx.ServerError("Unable to BeginRegistration", err)
		return
	}

	// Save the session data as marshaled JSON
	if err = ctx.Session.Set("webauthnRegistration", sessionData); err != nil {
		ctx.ServerError("Unable to set session", err)
		return
	}

	ctx.JSON(http.StatusOK, credentialOptions)
}

// WebauthnRegisterPost receives the response of the security key
func WebauthnRegisterPost(ctx *context.Context) {
	name, ok := ctx.Session.Get("webauthnName").(string)
	if !ok || name == "" {
		ctx.ServerError("Get webauthnName", errors.New("no webauthnName"))
		return
	}

	// Load the session data
	sessionData, ok := ctx.Session.Get("webauthnRegistration").(*webauthn.SessionData)
	if !ok || sessionData == nil {
		ctx.ServerError("Get registration", errors.New("no registration"))
		return
	}
	defer func() {
		_ = ctx.Session.Delete("webauthnRegistration")
	}()

	// Verify that the challenge succeeded
	cred, err := wa.WebAuthn.FinishRegistration((*wa.User)(ctx.Doer), *sessionData, ctx.Req)
	if err != nil {
		if pErr, ok := err.(*protocol.Error); ok {
			log.Error("Unable to finish registration due to error: %v\nDevInfo: %s", pErr, pErr.DevInfo)
		}
		ctx.ServerError("CreateCredential", err)
		return
	}

	dbCred, err := auth.GetWebAuthnCredentialByName(ctx.Doer.ID, name)
	if err != nil && !auth.IsErrWebAuthnCredentialNotExist(err) {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	if dbCred != nil {
		ctx.Error(http.StatusConflict, "Name already taken")
		return
	}

	// Create the credential
	_, err = auth.CreateCredential(ctx.Doer.ID, name, cred)
	if err != nil {
		ctx.ServerError("CreateCredential", err)
		return
	}
	_ = ctx.Session.Delete("webauthnName")

	ctx.JSON(http.StatusCreated, cred)
}

// WebauthnDelete deletes an security key by id
func WebauthnDelete(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.WebauthnDeleteForm)
	if _, err := auth.DeleteCredential(form.ID, ctx.Doer.ID); err != nil {
		ctx.ServerError("GetWebAuthnCredentialByID", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}
