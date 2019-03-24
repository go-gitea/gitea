// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"errors"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"

	"github.com/tstranex/u2f"
)

// U2FRegister initializes the u2f registration procedure
func U2FRegister(ctx *context.Context, form auth.U2FRegistrationForm) {
	if form.Name == "" {
		ctx.Error(409)
		return
	}
	challenge, err := u2f.NewChallenge(setting.U2F.AppID, setting.U2F.TrustedFacets)
	if err != nil {
		ctx.ServerError("NewChallenge", err)
		return
	}
	err = ctx.Session.Set("u2fChallenge", challenge)
	if err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	regs, err := models.GetU2FRegistrationsByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetU2FRegistrationsByUID", err)
		return
	}
	for _, reg := range regs {
		if reg.Name == form.Name {
			ctx.Error(409, "Name already taken")
			return
		}
	}
	err = ctx.Session.Set("u2fName", form.Name)
	if err != nil {
		ctx.ServerError("", err)
		return
	}
	ctx.JSON(200, u2f.NewWebRegisterRequest(challenge, regs.ToRegistrations()))
}

// U2FRegisterPost receives the response of the security key
func U2FRegisterPost(ctx *context.Context, response u2f.RegisterResponse) {
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
	reg, err := u2f.Register(response, *challenge, config)
	if err != nil {
		ctx.ServerError("u2f.Register", err)
		return
	}
	if _, err = models.CreateRegistration(ctx.User, name, reg); err != nil {
		ctx.ServerError("u2f.Register", err)
		return
	}
	ctx.Status(200)
}

// U2FDelete deletes an security key by id
func U2FDelete(ctx *context.Context, form auth.U2FDeleteForm) {
	reg, err := models.GetU2FRegistrationByID(form.ID)
	if err != nil {
		if models.IsErrU2FRegistrationNotExist(err) {
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
	if err := models.DeleteRegistration(reg); err != nil {
		ctx.ServerError("DeleteRegistration", err)
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}
