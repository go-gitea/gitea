// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/repo"
)

// GetUserByParamsName get user by name
func GetUserByParamsName(ctx *context.APIContext, name string) *models.User {
	user, err := models.GetUserByName(ctx.Params(name))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL paramenter.
func GetUserByParams(ctx *context.APIContext) *models.User {
	return GetUserByParamsName(ctx, ":username")
}

func composePublicKeysAPILink() string {
	return setting.AppURL + "api/v1/user/keys/"
}

func listPublicKeys(ctx *context.APIContext, uid int64) {
	keys, err := models.ListPublicKeys(uid)
	if err != nil {
		ctx.Error(500, "ListPublicKeys", err)
		return
	}

	apiLink := composePublicKeysAPILink()
	apiKeys := make([]*api.PublicKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToPublicKey(apiLink, keys[i])
	}

	ctx.JSON(200, &apiKeys)
}

// ListMyPublicKeys list all my public keys
// see https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#list-your-public-keys
func ListMyPublicKeys(ctx *context.APIContext) {
	listPublicKeys(ctx, ctx.User.ID)
}

// ListPublicKeys list all user's public keys
// see https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#list-public-keys-for-a-user
func ListPublicKeys(ctx *context.APIContext) {
	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listPublicKeys(ctx, user.ID)
}

// GetPublicKey get one public key
// see https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#get-a-single-public-key
func GetPublicKey(ctx *context.APIContext) {
	key, err := models.GetPublicKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrKeyNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPublicKeyByID", err)
		}
		return
	}

	apiLink := composePublicKeysAPILink()
	ctx.JSON(200, convert.ToPublicKey(apiLink, key))
}

// CreateUserPublicKey creates new public key to given user by ID.
func CreateUserPublicKey(ctx *context.APIContext, form api.CreateKeyOption, uid int64) {
	content, err := models.CheckPublicKeyString(form.Key)
	if err != nil {
		repo.HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := models.AddPublicKey(uid, form.Title, content)
	if err != nil {
		repo.HandleAddKeyError(ctx, err)
		return
	}
	apiLink := composePublicKeysAPILink()
	ctx.JSON(201, convert.ToPublicKey(apiLink, key))
}

// CreatePublicKey create one public key for me
// see https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#create-a-public-key
func CreatePublicKey(ctx *context.APIContext, form api.CreateKeyOption) {
	CreateUserPublicKey(ctx, form, ctx.User.ID)
}

// DeletePublicKey delete one public key of mine
// see https://github.com/gogits/go-gogs-client/wiki/Users-Public-Keys#delete-a-public-key
func DeletePublicKey(ctx *context.APIContext) {
	if err := models.DeletePublicKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrKeyAccessDenied(err) {
			ctx.Error(403, "", "You do not have access to this key")
		} else {
			ctx.Error(500, "DeletePublicKey", err)
		}
		return
	}

	ctx.Status(204)
}
