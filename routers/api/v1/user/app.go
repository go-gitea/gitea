// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// ListAccessTokens list all the access tokens
// see https://github.com/gogits/go-gogs-client/wiki/Users#list-access-tokens-for-a-user
func ListAccessTokens(ctx *context.APIContext) {
	tokens, err := models.ListAccessTokens(ctx.User.ID)
	if err != nil {
		ctx.Error(500, "ListAccessTokens", err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{tokens[i].Name, tokens[i].Sha1}
	}
	ctx.JSON(200, &apiTokens)
}

// CreateAccessToken create access tokens
// see https://github.com/gogits/go-gogs-client/wiki/Users#create-a-access-token
func CreateAccessToken(ctx *context.APIContext, form api.CreateAccessTokenOption) {
	t := &models.AccessToken{
		UID:  ctx.User.ID,
		Name: form.Name,
	}
	if err := models.NewAccessToken(t); err != nil {
		ctx.Error(500, "NewAccessToken", err)
		return
	}
	ctx.JSON(201, &api.AccessToken{t.Name, t.Sha1})
}
