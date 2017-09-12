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
func ListAccessTokens(ctx *context.APIContext) {
	// swagger:route GET /users/{username}/tokens user userGetTokens
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: AccessTokenList
	//       500: error

	tokens, err := models.ListAccessTokens(ctx.User.ID)
	if err != nil {
		ctx.Error(500, "ListAccessTokens", err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{
			Name: tokens[i].Name,
			Sha1: tokens[i].Sha1,
		}
	}
	ctx.JSON(200, &apiTokens)
}

// CreateAccessToken create access tokens
func CreateAccessToken(ctx *context.APIContext, form api.CreateAccessTokenOption) {
	// swagger:route POST /users/{username} /tokens user userCreateToken
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: AccessToken
	//       500: error

	t := &models.AccessToken{
		UID:  ctx.User.ID,
		Name: form.Name,
	}
	if err := models.NewAccessToken(t); err != nil {
		ctx.Error(500, "NewAccessToken", err)
		return
	}
	ctx.JSON(201, &api.AccessToken{
		Name: t.Name,
		Sha1: t.Sha1,
	})
}
