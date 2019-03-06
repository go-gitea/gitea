// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/util"

	macaron "gopkg.in/macaron.v1"
)

// UpdateDeployKey update deploy key updates
func UpdateDeployKey(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	deployKey, err := models.GetDeployKeyByRepo(keyID, repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	deployKey.UpdatedUnix = util.TimeStampNow()
	if err = models.UpdateDeployKeyCols(deployKey, "updated_unix"); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.PlainText(200, []byte("success"))
}

// UpdatePublicKey update publick key updates
func UpdatePublicKey(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	if err := models.UpdatePublicKeyUpdated(keyID); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.PlainText(200, []byte("success"))
}

//GetPublicKeyByID chainload to models.GetPublicKeyByID
func GetPublicKeyByID(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	key, err := models.GetPublicKeyByID(keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, key)
}

//GetUserByKeyID chainload to models.GetUserByKeyID
func GetUserByKeyID(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	user, err := models.GetUserByKeyID(keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, user)
}

//GetDeployKey chainload to models.GetDeployKey
func GetDeployKey(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	dKey, err := models.GetDeployKeyByRepo(keyID, repoID)
	if err != nil {
		if models.IsErrDeployKeyNotExist(err) {
			ctx.JSON(404, []byte("not found"))
			return
		}
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, dKey)
}

//HasDeployKey chainload to models.HasDeployKey
func HasDeployKey(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	if models.HasDeployKey(keyID, repoID) {
		ctx.PlainText(200, []byte("success"))
		return
	}
	ctx.PlainText(404, []byte("not found"))
}
