// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/timeutil"

	"gitea.com/macaron/macaron"
)

// UpdatePublicKeyInRepo update public key and deploy key updates
func UpdatePublicKeyInRepo(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	repoID := ctx.ParamsInt64(":repoid")
	if err := models.UpdatePublicKeyUpdated(keyID); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	deployKey, err := models.GetDeployKeyByRepo(keyID, repoID)
	if err != nil {
		if models.IsErrDeployKeyNotExist(err) {
			ctx.PlainText(200, []byte("success"))
			return
		}
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	deployKey.UpdatedUnix = timeutil.TimeStampNow()
	if err = models.UpdateDeployKeyCols(deployKey, "updated_unix"); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.PlainText(http.StatusOK, []byte("success"))
}

// AuthorizedPublicKeyByContent searches content as prefix (leak e-mail part)
// and returns public key found.
func AuthorizedPublicKeyByContent(ctx *macaron.Context) {
	content := ctx.Query("content")

	publicKey, err := models.SearchPublicKeyByContent(content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.PlainText(http.StatusOK, []byte(publicKey.AuthorizedString()))
}
