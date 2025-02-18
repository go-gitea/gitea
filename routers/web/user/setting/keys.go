// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplSettingsKeys templates.TplName = "user/settings/keys"
)

// Keys render user's SSH/GPG public keys page
func Keys(ctx *context.Context) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys, setting.UserFeatureManageGPGKeys) {
		ctx.NotFound(fmt.Errorf("keys setting is not allowed to be changed"))
		return
	}

	ctx.Data["Title"] = ctx.Tr("settings.ssh_gpg_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["BuiltinSSH"] = setting.SSH.StartBuiltinServer
	ctx.Data["AllowPrincipals"] = setting.SSH.AuthorizedPrincipalsEnabled
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)

	loadKeysData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsKeys)
}

// KeysPost response for change user's SSH/GPG keys
func KeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["BuiltinSSH"] = setting.SSH.StartBuiltinServer
	ctx.Data["AllowPrincipals"] = setting.SSH.AuthorizedPrincipalsEnabled
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)

	if ctx.HasError() {
		loadKeysData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsKeys)
		return
	}
	switch form.Type {
	case "principal":
		content, err := asymkey_model.CheckPrincipalKeyString(ctx, ctx.Doer, form.Content)
		if err != nil {
			if db.IsErrSSHDisabled(err) {
				ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
			} else {
				ctx.Flash.Error(ctx.Tr("form.invalid_ssh_principal", err.Error()))
			}
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}
		if _, err = asymkey_service.AddPrincipalKey(ctx, ctx.Doer.ID, content, 0); err != nil {
			ctx.Data["HasPrincipalError"] = true
			switch {
			case asymkey_model.IsErrKeyAlreadyExist(err), asymkey_model.IsErrKeyNameAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_principal_been_used"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("AddPrincipalKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_principal_success", form.Content))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "gpg":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
			ctx.NotFound(fmt.Errorf("gpg keys setting is not allowed to be visited"))
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
			switch {
			case asymkey_model.IsErrGPGKeyParsing(err):
				ctx.Flash.Error(ctx.Tr("form.invalid_gpg_key", err.Error()))
				ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			case asymkey_model.IsErrGPGKeyIDAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.gpg_key_id_used"), tplSettingsKeys, &form)
			case asymkey_model.IsErrGPGInvalidTokenSignature(err):
				loadKeysData(ctx)
				ctx.Data["Err_Content"] = true
				ctx.Data["Err_Signature"] = true
				keyID := err.(asymkey_model.ErrGPGInvalidTokenSignature).ID
				ctx.Data["KeyID"] = keyID
				ctx.Data["PaddedKeyID"] = asymkey_model.PaddedKeyID(keyID)
				ctx.RenderWithErr(ctx.Tr("settings.gpg_invalid_token_signature"), tplSettingsKeys, &form)
			case asymkey_model.IsErrGPGNoEmailFound(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.Data["Err_Signature"] = true
				keyID := err.(asymkey_model.ErrGPGNoEmailFound).ID
				ctx.Data["KeyID"] = keyID
				ctx.Data["PaddedKeyID"] = asymkey_model.PaddedKeyID(keyID)
				ctx.RenderWithErr(ctx.Tr("settings.gpg_no_key_email_found"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("AddPublicKey", err)
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
	case "verify_gpg":
		token := asymkey_model.VerificationToken(ctx.Doer, 1)
		lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

		keyID, err := asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, token, form.Signature)
		if err != nil && asymkey_model.IsErrGPGInvalidTokenSignature(err) {
			keyID, err = asymkey_model.VerifyGPGKey(ctx, ctx.Doer.ID, form.KeyID, lastToken, form.Signature)
		}
		if err != nil {
			ctx.Data["HasGPGVerifyError"] = true
			switch {
			case asymkey_model.IsErrGPGInvalidTokenSignature(err):
				loadKeysData(ctx)
				ctx.Data["VerifyingID"] = form.KeyID
				ctx.Data["Err_Signature"] = true
				keyID := err.(asymkey_model.ErrGPGInvalidTokenSignature).ID
				ctx.Data["KeyID"] = keyID
				ctx.Data["PaddedKeyID"] = asymkey_model.PaddedKeyID(keyID)
				ctx.RenderWithErr(ctx.Tr("settings.gpg_invalid_token_signature"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("VerifyGPG", err)
			}
		}
		ctx.Flash.Success(ctx.Tr("settings.verify_gpg_key_success", keyID))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.NotFound(fmt.Errorf("ssh keys setting is not allowed to be visited"))
			return
		}

		content, err := asymkey_model.CheckPublicKeyString(form.Content)
		if err != nil {
			if db.IsErrSSHDisabled(err) {
				ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
			} else if asymkey_model.IsErrKeyUnableVerify(err) {
				ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
			} else if err == asymkey_model.ErrKeyIsPrivate {
				ctx.Flash.Error(ctx.Tr("form.must_use_public_key"))
			} else {
				ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
			}
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}

		if _, err = asymkey_model.AddPublicKey(ctx, ctx.Doer.ID, form.Title, content, 0); err != nil {
			ctx.Data["HasSSHError"] = true
			switch {
			case asymkey_model.IsErrKeyAlreadyExist(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplSettingsKeys, &form)
			case asymkey_model.IsErrKeyNameAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Title"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_key_name_used"), tplSettingsKeys, &form)
			case asymkey_model.IsErrKeyUnableVerify(err):
				ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
				ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			default:
				ctx.ServerError("AddPublicKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_key_success", form.Title))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "verify_ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.NotFound(fmt.Errorf("ssh keys setting is not allowed to be visited"))
			return
		}

		token := asymkey_model.VerificationToken(ctx.Doer, 1)
		lastToken := asymkey_model.VerificationToken(ctx.Doer, 0)

		fingerprint, err := asymkey_model.VerifySSHKey(ctx, ctx.Doer.ID, form.Fingerprint, token, form.Signature)
		if err != nil && asymkey_model.IsErrSSHInvalidTokenSignature(err) {
			fingerprint, err = asymkey_model.VerifySSHKey(ctx, ctx.Doer.ID, form.Fingerprint, lastToken, form.Signature)
		}
		if err != nil {
			ctx.Data["HasSSHVerifyError"] = true
			switch {
			case asymkey_model.IsErrSSHInvalidTokenSignature(err):
				loadKeysData(ctx)
				ctx.Data["Err_Signature"] = true
				ctx.Data["Fingerprint"] = err.(asymkey_model.ErrSSHInvalidTokenSignature).Fingerprint
				ctx.RenderWithErr(ctx.Tr("settings.ssh_invalid_token_signature"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("VerifySSH", err)
			}
		}
		ctx.Flash.Success(ctx.Tr("settings.verify_ssh_key_success", fingerprint))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")

	default:
		ctx.Flash.Warning("Function not implemented")
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	}
}

// DeleteKey response for delete user's SSH/GPG key
func DeleteKey(ctx *context.Context) {
	switch ctx.FormString("type") {
	case "gpg":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageGPGKeys) {
			ctx.NotFound(fmt.Errorf("gpg keys setting is not allowed to be visited"))
			return
		}
		if err := asymkey_model.DeleteGPGKey(ctx, ctx.Doer, ctx.FormInt64("id")); err != nil {
			ctx.Flash.Error("DeleteGPGKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.gpg_key_deletion_success"))
		}
	case "ssh":
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageSSHKeys) {
			ctx.NotFound(fmt.Errorf("ssh keys setting is not allowed to be visited"))
			return
		}

		keyID := ctx.FormInt64("id")
		external, err := asymkey_model.PublicKeyIsExternallyManaged(ctx, keyID)
		if err != nil {
			ctx.ServerError("sshKeysExternalManaged", err)
			return
		}
		if external {
			ctx.Flash.Error(ctx.Tr("settings.ssh_externally_managed"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}
		if err := asymkey_service.DeletePublicKey(ctx, ctx.Doer, keyID); err != nil {
			ctx.Flash.Error("DeletePublicKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.ssh_key_deletion_success"))
		}
	case "principal":
		if err := asymkey_service.DeletePublicKey(ctx, ctx.Doer, ctx.FormInt64("id")); err != nil {
			ctx.Flash.Error("DeletePublicKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.ssh_principal_deletion_success"))
		}
	default:
		ctx.Flash.Warning("Function not implemented")
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
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

	// generate a new aes cipher using the csrfToken
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
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)
}
