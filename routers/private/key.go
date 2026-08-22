// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	deploykey_model "gitea.dev/models/deploykey"
	"gitea.dev/services/context"
)

// UpdatePublicKeyInRepo update public key and deploy key updates
func UpdatePublicKeyInRepo(ctx *context.PrivateContext) {
	keyID := ctx.PathParamInt64("id")
	repoID := ctx.PathParamInt64("repoid")
	if err := asymkey_model.UpdatePublicKeyUpdated(ctx, keyID); err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}

	deployKey, err := deploykey_model.GetDeployKeyByRepoPublicKey(ctx, repoID, keyID)
	if err != nil {
		if deploykey_model.IsErrDeployKeyNotExist(err) {
			ctx.PlainText(http.StatusOK, "success")
			return
		}
		ctx.PrivateInternalErrorf("%v", err)
		return
	}
	if err = deploykey_model.UpdateDeployKeyUpdated(ctx, deployKey.ID); err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}

	ctx.PlainText(http.StatusOK, "success")
}

// AuthorizedPublicKeyByContent searches content as prefix (without comment part)
// and returns public key found.
func AuthorizedPublicKeyByContent(ctx *context.PrivateContext) {
	content := ctx.FormString("content")

	publicKey, err := asymkey_model.SearchPublicKeyByContent(ctx, content)
	if err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}

	authorizedString, err := asymkey_model.AuthorizedStringForKey(publicKey)
	if err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}
	ctx.PlainText(http.StatusOK, authorizedString)
}
