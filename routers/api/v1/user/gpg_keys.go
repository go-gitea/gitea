// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/context"
	"github.com/gogits/gogs/modules/setting"
	"github.com/gogits/gogs/routers/api/v1/convert"
	"github.com/gogits/gogs/routers/api/v1/repo"
)

func composePublicGPGKeysAPILink() string {
	return setting.AppUrl + "api/v1/user/gpg_keys/"
}

func listPublicGPGKeys(ctx *context.APIContext, uid int64) {
	keys, err := models.ListPublicGPGKeys(uid)
	if err != nil {
		ctx.Error(500, "ListPublicGPGKeys", err)
		return
	}

	apiLink := composePublicGPGKeysAPILink()
	apiKeys := make([]*api.PublicKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToPublicGPGKey(apiLink, keys[i])
	}

	ctx.JSON(200, &apiKeys)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-GPG-Keys#list-your-public-keys
func ListMyPublicGPGKeys(ctx *context.APIContext) {
	listPublicGPGKeys(ctx, ctx.User.Id)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-GPG-Keys#get-a-single-public-key
func GetPublicGPGKey(ctx *context.APIContext) {
	key, err := models.GetPublicGPGKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrKeyNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPublicGPGKeyByID", err)
		}
		return
	}

	apiLink := composePublicGPGKeysAPILink()
	ctx.JSON(200, convert.ToPublicGPGKey(apiLink, key))
}

// CreateUserPublicGPGKey creates new public GPG key to given user by ID.
func CreateUserPublicGPGKey(ctx *context.APIContext, form api.CreateKeyOption, uid int64) {
	content, err := models.CheckPublicGPGKeyString(form.Key)
	if err != nil {
		repo.HandleCheckGPGKeyStringError(ctx, err)
		return
	}

	key, err := models.AddPublicGPGKey(uid, form.Title, content)
	if err != nil {
		repo.HandleAddGPGKeyError(ctx, err)
		return
	}
	apiLink := composePublicKeysAPILink()
	ctx.JSON(201, convert.ToPublicKey(apiLink, key))
}

//TODO Update api
// https://github.com/gogits/go-gogs-client/wiki/Users-Public-GPG-Keys#create-a-public-key
func CreatePublicGPGKey(ctx *context.APIContext, form api.CreateKeyOption) {
	CreateUserPublicGPGKey(ctx, form, ctx.User.Id)
}

// https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#delete-a-public-key
func DeletePublicGPGKey(ctx *context.APIContext) {
	if err := models.DeletePublicGPGKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrGPGKeyAccessDenied(err) {
			ctx.Error(403, "", "You do not have access to this key")
		} else {
			ctx.Error(500, "DeletePublicGPGKey", err)
		}
		return
	}

	ctx.Status(204)
}
