// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"html/template"
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplSettingsKeys templates.TplName = "user/settings/keys"
)

// Keys render user's SSH/GPG public keys page
func Keys(ctx *context.Context) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys, setting.UserFeatureManageGPGKeys) {
		ctx.NotFound(errors.New("keys setting is not allowed to be changed"))
		return
	}

	renderKeysPage(ctx)
}

// KeysPost response for change user's SSH/GPG keys
func KeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)

	// the "gpg" add form still answers with HTML, it reveals its token signature step by re-rendering
	if form.Type == "gpg" {
		addGPGKeyPost(ctx, form)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}
	switch form.Type {
	case "principal":
		content, err := asymkey_model.CheckPrincipalKeyString(ctx, ctx.Doer, form.Content)
		if err != nil {
			if db.IsErrSSHDisabled(err) {
				ctx.JSONError(ctx.Tr("settings.ssh_disabled"))
			} else {
				ctx.JSONError(ctx.Tr("form.invalid_ssh_principal", err.Error()))
			}
			return
		}
		if _, err = asymkey_service.AddPrincipalKey(ctx, ctx.Doer.ID, content, 0); err != nil {
			switch {
			case asymkey_model.IsErrKeyAlreadyExist(err), asymkey_model.IsErrKeyNameAlreadyUsed(err):
				ctx.JSONError(ctx.Tr("settings.ssh_principal_been_used"))
			default:
				ctx.ServerError("AddPrincipalKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_principal_success", form.Content))
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/keys")
	case "verify_gpg":
		token := asymkey_model.VerificationToken(ctx.Doer, 1)
		lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

		keyID, err := asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, token, form.Signature)
		if err != nil && asymkey_model.IsErrGPGInvalidTokenSignature(err) {
			keyID, err = asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, lastToken, form.Signature)
		}
		if err != nil {
			switch {
			case asymkey_model.IsErrGPGInvalidTokenSignature(err):
				ctx.JSONError(ctx.Tr("settings.gpg_invalid_token_signature"))
			default:
				ctx.ServerError("VerifyGPG", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.verify_gpg_key_success", keyID))
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/keys")
	case "ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.NotFound(errors.New("ssh keys setting is not allowed to be visited"))
			return
		}

		content, err := asymkey_model.CheckPublicKeyString(form.Content)
		if err != nil {
			switch {
			case db.IsErrSSHDisabled(err):
				ctx.JSONError(ctx.Tr("settings.ssh_disabled"))
			case asymkey_model.IsErrKeyUnableVerify(err):
				ctx.JSONError(ctx.Tr("form.unable_verify_ssh_key"))
			case errors.Is(err, asymkey_model.ErrKeyIsPrivate):
				ctx.JSONError(ctx.Tr("form.must_use_public_key"))
			default:
				ctx.JSONError(ctx.Tr("form.invalid_ssh_key", err.Error()))
			}
			return
		}

		if _, err = asymkey_model.AddPublicKey(ctx, ctx.Doer.ID, form.Title, content, 0, false); err != nil {
			switch {
			case asymkey_model.IsErrKeyAlreadyExist(err):
				ctx.JSONError(ctx.Tr("settings.ssh_key_been_used"))
			case asymkey_model.IsErrKeyNameAlreadyUsed(err):
				ctx.JSONError(ctx.Tr("settings.ssh_key_name_used"))
			case asymkey_model.IsErrKeyUnableVerify(err):
				ctx.JSONError(ctx.Tr("form.unable_verify_ssh_key"))
			default:
				ctx.ServerError("AddPublicKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_key_success", form.Title))
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/keys")
	case "verify_ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.NotFound(errors.New("ssh keys setting is not allowed to be visited"))
			return
		}

		token := asymkey_model.VerificationToken(ctx.Doer, 1)
		lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

		fingerprint, err := asymkey_model.VerifySSHKey(ctx, ctx.Doer.ID, form.Fingerprint, token, form.Signature)
		if err != nil && asymkey_model.IsErrSSHInvalidTokenSignature(err) {
			fingerprint, err = asymkey_model.VerifySSHKey(ctx, ctx.Doer.ID, form.Fingerprint, lastToken, form.Signature)
		}
		if err != nil {
			switch {
			case asymkey_model.IsErrSSHInvalidTokenSignature(err):
				ctx.JSONError(ctx.Tr("settings.ssh_invalid_token_signature"))
			default:
				ctx.ServerError("VerifySSH", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.verify_ssh_key_success", fingerprint))
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/keys")

	default:
		ctx.JSONError("Function not implemented")
	}
}

func addGPGKeyPost(ctx *context.Context, form *forms.AddKeyForm) {
	if ctx.HasError() {
		renderKeysPage(ctx)
		return
	}

	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
		ctx.NotFound(errors.New("gpg keys setting is not allowed to be visited"))
		return
	}

	token := asymkey_model.VerificationToken(ctx.Doer, 1)
	lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

	keys, err := asymkey_model.AddGPGKey(ctx, ctx.Doer.ID, form.Content, token, form.Signature)
	if err != nil && asymkey_model.IsErrGPGInvalidTokenSignature(err) {
		keys, err = asymkey_model.AddGPGKey(ctx, ctx.Doer.ID, form.Content, lastToken, form.Signature)
	}
	if err != nil {
		ctx.Data["HasGPGError"] = true
		ctx.Data["Err_Content"] = true
		switch {
		case asymkey_model.IsErrGPGKeyParsing(err):
			ctx.Flash.Error(ctx.Tr("form.invalid_gpg_key", err.Error()))
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
		case asymkey_model.IsErrGPGKeyIDAlreadyUsed(err):
			renderKeysPageWithErr(ctx, ctx.Tr("settings.gpg_key_id_used"), form)
		case asymkey_model.IsErrGPGInvalidTokenSignature(err):
			ctx.Data["Err_Signature"] = true
			ctx.Data["PaddedKeyID"] = asymkey_model.PaddedKeyID(err.(asymkey_model.ErrGPGInvalidTokenSignature).ID)
			renderKeysPageWithErr(ctx, ctx.Tr("settings.gpg_invalid_token_signature"), form)
		case asymkey_model.IsErrGPGNoEmailFound(err):
			ctx.Data["Err_Signature"] = true
			ctx.Data["PaddedKeyID"] = asymkey_model.PaddedKeyID(err.(asymkey_model.ErrGPGNoEmailFound).ID)
			renderKeysPageWithErr(ctx, ctx.Tr("settings.gpg_no_key_email_found"), form)
		default:
			ctx.ServerError("AddGPGKey", err)
		}
		return
	}

	keyIDs := ""
	for _, key := range keys {
		keyIDs += key.KeyID
		keyIDs += ", "
	}
	if len(keyIDs) > 0 {
		keyIDs = keyIDs[:len(keyIDs)-2]
	}
	ctx.Flash.Success(ctx.Tr("settings.add_gpg_key_success", keyIDs))
	ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
}

func renderKeysPage(ctx *context.Context) {
	if !prepareKeysPageData(ctx) {
		return
	}
	ctx.HTML(http.StatusOK, tplSettingsKeys)
}

func renderKeysPageWithErr(ctx *context.Context, msg template.HTML, form *forms.AddKeyForm) {
	if !prepareKeysPageData(ctx) {
		return
	}
	ctx.RenderWithErrDeprecated(msg, tplSettingsKeys, form)
}

func prepareKeysPageData(ctx *context.Context) bool {
	ctx.Data["Title"] = ctx.Tr("settings.ssh_gpg_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["AllowPrincipals"] = setting.SSH.AuthorizedPrincipalsEnabled
	loadKeysData(ctx)
	return !ctx.Written()
}

// DeleteKey response for delete user's SSH/GPG key
func DeleteKey(ctx *context.Context) {
	switch ctx.FormString("type") {
	case "gpg":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
			ctx.JSONError("gpg keys setting is not allowed to be visited")
			return
		}
		if err := asymkey_model.DeleteGPGKey(ctx, ctx.Doer, ctx.FormInt64("id")); err != nil {
			ctx.JSONError("Failed to delete PGP key")
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.gpg_key_deletion_success"))
	case "ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.JSONError("ssh keys setting is not allowed to be visited")
			return
		}

		keyID := ctx.FormInt64("id")
		external, err := asymkey_model.PublicKeyIsExternallyManaged(ctx, keyID)
		if err != nil {
			ctx.ServerError("sshKeysExternalManaged", err)
			return
		}
		if external {
			ctx.JSONError(ctx.Tr("settings.ssh_externally_managed"))
			return
		}
		if err := asymkey_service.DeletePublicKey(ctx, ctx.Doer, keyID); err != nil {
			ctx.JSONError("Failed to delete SSH key")
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.ssh_key_deletion_success"))
	case "principal":
		if err := asymkey_service.DeletePublicKey(ctx, ctx.Doer, ctx.FormInt64("id")); err != nil {
			ctx.JSONError("Failed to delete SSH principal key")
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.ssh_principal_deletion_success"))
	default:
		ctx.JSONError("unsupported key type")
		return
	}
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/keys")
}

func loadKeysData(ctx *context.Context) {
	keys, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
		OwnerID:    ctx.Doer.ID,
		NotKeytype: asymkey_model.KeyTypePrincipal,
	})
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}
	ctx.Data["Keys"] = keys

	externalKeys, err := asymkey_model.PublicKeysAreExternallyManaged(ctx, keys)
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}
	ctx.Data["ExternalKeys"] = externalKeys

	gpgkeys, err := db.Find[asymkey_model.GPGKey](ctx, asymkey_model.FindGPGKeyOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     ctx.Doer.ID,
	})
	if err != nil {
		ctx.ServerError("ListGPGKeys", err)
		return
	}
	if err := asymkey_model.GPGKeyList(gpgkeys).LoadSubKeys(ctx); err != nil {
		ctx.ServerError("LoadSubKeys", err)
		return
	}
	ctx.Data["GPGKeys"] = gpgkeys
	tokenToSign := asymkey_model.VerificationToken(ctx.Doer, 1)

	// generate a new aes cipher using the token
	ctx.Data["TokenToSign"] = tokenToSign

	principals, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     ctx.Doer.ID,
		KeyTypes:    []asymkey_model.KeyType{asymkey_model.KeyTypePrincipal},
	})
	if err != nil {
		ctx.ServerError("ListPrincipalKeys", err)
		return
	}
	ctx.Data["Principals"] = principals

	ctx.Data["VerifyingID"] = ctx.FormString("verify_gpg")
	ctx.Data["VerifyingFingerprint"] = ctx.FormString("verify_ssh")
}
