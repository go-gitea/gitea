// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	wa "code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"
)

// WebAuthnRegister initializes the webauthn registration procedure
func WebAuthnRegister(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.WebauthnRegistrationForm)
	if form.Name == "" {
		ctx.Error(http.StatusConflict)
		return
	}
	creds, err := auth.GetWebAuthnCredentialsByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	for _, reg := range creds {
		if strings.EqualFold(reg.Name, form.Name) {
			ctx.Error(http.StatusConflict, "Name already taken")
			return
		}
	}
	_ = ctx.Session.Delete("registration")
	if err := ctx.Session.Set("WebauthnName", form.Name); err != nil {
		ctx.ServerError("Unable to set session key for WebauthnName", err)
		return
	}

	var residentKeyRequirement *bool
	if strings.EqualFold(form.ResKey, "true") {
		residentKeyRequirement = protocol.ResidentKeyRequired()
	} else {
		residentKeyRequirement = protocol.ResidentKeyUnrequired()
	}

	testEx := protocol.AuthenticationExtensions(map[string]interface{}{"txAuthSimple": form.TxAuthSimpleExtension})

	credentialOptions, sessionData, err := wa.WebAuthn.BeginRegistration((*wa.User)(ctx.User),
		webauthn.WithAuthenticatorSelection(
			protocol.AuthenticatorSelection{
				AuthenticatorAttachment: protocol.AuthenticatorAttachment(form.AuthType),
				RequireResidentKey:      residentKeyRequirement,
				UserVerification:        protocol.UserVerificationRequirement(form.UserVer),
			}),
		webauthn.WithConveyancePreference(protocol.ConveyancePreference(form.AttType)),
		webauthn.WithExtensions(testEx),
	)
	if err != nil {
		ctx.ServerError("Unable to BeginRegistration", err)
		return
	}

	// Save the session data as marshaled JSON
	if err = ctx.Session.Set("registration", sessionData); err != nil {
		ctx.ServerError("Unable to set session", err)
		return
	}

	ctx.JSON(http.StatusOK, credentialOptions)
}

// WebauthnRegisterPost receives the response of the security key
func WebauthnRegisterPost(ctx *context.Context) {
	name := ctx.Session.Get("WebauthnName")
	if name == nil {
		ctx.ServerError("Get WebauthnName", errors.New("no WebauthnName"))
		return
	}

	// Load the session data
	sessionData := ctx.Session.Get("registration")
	if sessionData == nil {
		ctx.ServerError("Get registration", errors.New("no registration"))
		return
	}
	defer func() {
		_ = ctx.Session.Delete("registration")
	}()

	// Verify that the challenge succeeded
	cred, err := wa.WebAuthn.FinishRegistration((*wa.User)(ctx.User), *sessionData.(*webauthn.SessionData), ctx.Req)
	if err != nil {
		ctx.ServerError("CreateCredential", err)
		return
	}
	// If needed, you can perform additional checks here to ensure the
	// authenticator and generated credential conform to your requirements.

	// For our use case, we're encoding the raw credential ID as URL-safe
	// base64 since we anticipate rendering it in templates. If you choose to
	// do this, make sure to decode the credential ID before passing it back to
	// the webauthn library.
	_, err = auth.CreateCredential(ctx.User.ID, name.(string), cred)
	if err != nil {
		ctx.ServerError("CreateCredential", err)
		return
	}
	ctx.JSON(http.StatusCreated, cred)
}

// WebauthnDelete deletes an security key by id
func WebauthnDelete(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.WebauthnDeleteForm)
	reg, err := auth.GetWebAuthnCredentialByID(form.ID)
	if err != nil {
		if auth.IsErrWebAuthnCredentialNotExist(err) {
			ctx.Status(200)
			return
		}
		ctx.ServerError("GetWebAuthnCredentialByID", err)
		return
	}
	if reg.UserID != ctx.User.ID {
		ctx.Status(401)
		return
	}
	if err := auth.DeleteCredential(reg); err != nil {
		ctx.ServerError("DeleteRegistration", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}
