// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsSecurity    base.TplName = "user/settings/security/security"
	tplSettingsTwofaEnroll base.TplName = "user/settings/security/twofa_enroll"
)

// Security render change user's password page and 2FA
func Security(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true
	ctx.Data["RequireU2F"] = true

	if ctx.FormString("openid.return_to") != "" {
		settingsOpenIDVerify(ctx)
		return
	}

	loadSecurityData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsSecurity)
}

// DeleteAccountLink delete a single account link
func DeleteAccountLink(ctx *context.Context) {
	id := ctx.FormInt64("id")
	if id <= 0 {
		ctx.Flash.Error("Account link id is not given")
	} else {
		if _, err := user_model.RemoveAccountLink(ctx.User, id); err != nil {
			ctx.Flash.Error("RemoveAccountLink: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.remove_account_link_success"))
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}

func loadSecurityData(ctx *context.Context) {
	enrolled, err := auth.HasTwoFactorByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	ctx.Data["TOTPEnrolled"] = enrolled

	credentials, err := auth.GetWebAuthnCredentialsByUID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	ctx.Data["WebAuthnCredentials"] = credentials

	tokens, err := models.ListAccessTokens(models.ListAccessTokensOptions{UserID: ctx.User.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens

	accountLinks, err := user_model.ListAccountLinks(ctx.User)
	if err != nil {
		ctx.ServerError("ListAccountLinks", err)
		return
	}

	// map the provider display name with the AuthSource
	sources := make(map[*auth.Source]string)
	for _, externalAccount := range accountLinks {
		if authSource, err := auth.GetSourceByID(externalAccount.LoginSourceID); err == nil {
			var providerDisplayName string

			type DisplayNamed interface {
				DisplayName() string
			}

			type Named interface {
				Name() string
			}

			if displayNamed, ok := authSource.Cfg.(DisplayNamed); ok {
				providerDisplayName = displayNamed.DisplayName()
			} else if named, ok := authSource.Cfg.(Named); ok {
				providerDisplayName = named.Name()
			} else {
				providerDisplayName = authSource.Name
			}
			sources[authSource] = providerDisplayName
		}
	}
	ctx.Data["AccountLinks"] = sources

	openid, err := user_model.GetUserOpenIDs(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openid
}
