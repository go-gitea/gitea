// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/tstranex/u2f"
)

// U2FRegister initializes the u2f registration procedure
func U2FRegister(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.U2FRegistrationForm)
	if form.Name == "" {
		ctx.Error(http.StatusConflict)
		return
	}
	challenge, err := u2f.NewChallenge(setting.U2F.AppID, setting.U2F.TrustedFacets)
	if err != nil {
		ctx.ServerError("NewChallenge", err)
		return
	}
	if err := ctx.Session.Set("u2fChallenge", challenge); err != nil {
		ctx.ServerError("Unable to set session key for u2fChallenge", err)
		return
	}
	regs, err := auth.GetU2FRegistrationsByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetU2FRegistrationsByUID", err)
		return
	}
	for _, reg := range regs {
		if reg.Name == form.Name {
			ctx.Error(http.StatusConflict, "Name already taken")
			return
		}
	}
	if err := ctx.Session.Set("u2fName", form.Name); err != nil {
		ctx.ServerError("Unable to set session key for u2fName", err)
		return
	}
	// Here we're just going to try to release the session early
	if err := ctx.Session.Release(); err != nil {
		// we'll tolerate errors here as they *should* get saved elsewhere
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.JSON(http.StatusOK, u2f.NewWebRegisterRequest(challenge, regs.ToRegistrations()))
}

// U2FRegisterPost receives the response of the security key
func U2FRegisterPost(ctx *context.Context) {
	response := web.GetForm(ctx).(*u2f.RegisterResponse)
	challSess := ctx.Session.Get("u2fChallenge")
	u2fName := ctx.Session.Get("u2fName")
	if challSess == nil || u2fName == nil {
		ctx.ServerError("U2FRegisterPost", errors.New("not in U2F session"))
		return
	}
	challenge := challSess.(*u2f.Challenge)
	name := u2fName.(string)
	config := &u2f.Config{
		// Chrome 66+ doesn't return the device's attestation
		// certificate by default.
		SkipAttestationVerify: true,
	}
	reg, err := u2f.Register(*response, *challenge, config)
	if err != nil {
		ctx.ServerError("u2f.Register", err)
		return
	}
	if _, err = auth.CreateRegistration(ctx.User.ID, name, reg); err != nil {
		ctx.ServerError("u2f.Register", err)
		return
	}
	ctx.Status(200)
}

// U2FDelete deletes an security key by id
func U2FDelete(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.U2FDeleteForm)
	reg, err := auth.GetU2FRegistrationByID(form.ID)
	if err != nil {
		if auth.IsErrU2FRegistrationNotExist(err) {
			ctx.Status(200)
			return
		}
		ctx.ServerError("GetU2FRegistrationByID", err)
		return
	}
	if reg.UserID != ctx.User.ID {
		ctx.Status(401)
		return
	}
	if err := auth.DeleteRegistration(reg); err != nil {
		ctx.ServerError("DeleteRegistration", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}
