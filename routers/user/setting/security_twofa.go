// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"image/png"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// RegenerateScratchTwoFactor regenerates the user's 2FA scratch code.
func RegenerateScratchTwoFactor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true

	t, err := models.GetTwoFactorByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	token, err := t.GenerateScratchToken()
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	if err = models.UpdateTwoFactor(t); err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.twofa_scratch_token_regenerated", token))
	ctx.Redirect(setting.AppSubURL + "/user/settings/security")
}

// DisableTwoFactor deletes the user's 2FA settings.
func DisableTwoFactor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true

	t, err := models.GetTwoFactorByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	if err = models.DeleteTwoFactorByID(t.ID, ctx.User.ID); err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.twofa_disabled"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/security")
}

func twofaGenerateSecretAndQr(ctx *context.Context) bool {
	var otpKey *otp.Key
	var err error
	uri := ctx.Session.Get("twofaUri")
	if uri != nil {
		otpKey, err = otp.NewKeyFromURL(uri.(string))
		if err != nil {
			ctx.ServerError("SettingsTwoFactor: NewKeyFromURL: ", err)
			return false
		}
	}
	// Filter unsafe character ':' in issuer
	issuer := strings.Replace(setting.AppName+" ("+setting.Domain+")", ":", "", -1)
	if otpKey == nil {
		otpKey, err = totp.Generate(totp.GenerateOpts{
			SecretSize:  40,
			Issuer:      issuer,
			AccountName: ctx.User.Name,
		})
		if err != nil {
			ctx.ServerError("SettingsTwoFactor", err)
			return false
		}
	}

	ctx.Data["TwofaSecret"] = otpKey.Secret()
	img, err := otpKey.Image(320, 240)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return false
	}

	var imgBytes bytes.Buffer
	if err = png.Encode(&imgBytes, img); err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return false
	}

	ctx.Data["QrUri"] = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes.Bytes()))
	err = ctx.Session.Set("twofaSecret", otpKey.Secret())
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return false
	}
	err = ctx.Session.Set("twofaUri", otpKey.String())
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return false
	}
	return true
}

// EnrollTwoFactor shows the page where the user can enroll into 2FA.
func EnrollTwoFactor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true

	t, err := models.GetTwoFactorByUID(ctx.User.ID)
	if t != nil {
		// already enrolled
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	if err != nil && !models.IsErrTwoFactorNotEnrolled(err) {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	if !twofaGenerateSecretAndQr(ctx) {
		return
	}

	ctx.HTML(200, tplSettingsTwofaEnroll)
}

// EnrollTwoFactorPost handles enrolling the user into 2FA.
func EnrollTwoFactorPost(ctx *context.Context, form auth.TwoFactorAuthForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true

	t, err := models.GetTwoFactorByUID(ctx.User.ID)
	if t != nil {
		// already enrolled
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	if err != nil && !models.IsErrTwoFactorNotEnrolled(err) {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	if ctx.HasError() {
		if !twofaGenerateSecretAndQr(ctx) {
			return
		}
		ctx.HTML(200, tplSettingsTwofaEnroll)
		return
	}

	secret := ctx.Session.Get("twofaSecret").(string)
	if !totp.Validate(form.Passcode, secret) {
		if !twofaGenerateSecretAndQr(ctx) {
			return
		}
		ctx.Flash.Error(ctx.Tr("settings.passcode_invalid"))
		ctx.HTML(200, tplSettingsTwofaEnroll)
		return
	}

	t = &models.TwoFactor{
		UID: ctx.User.ID,
	}
	err = t.SetSecret(secret)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	token, err := t.GenerateScratchToken()
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	if err = models.NewTwoFactor(t); err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}

	err = ctx.Session.Delete("twofaSecret")
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	err = ctx.Session.Delete("twofaUri")
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.twofa_enrolled", token))
	ctx.Redirect(setting.AppSubURL + "/user/settings/security")
}
