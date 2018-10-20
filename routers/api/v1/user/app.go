// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
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
	// swagger:operation GET /users/{username}/tokens user userGetTokens
	// ---
	// summary: List the authenticated user's access tokens
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AccessTokenList"
	tokens, err := models.ListAccessTokens(ctx.User.ID)
	if err != nil {
		ctx.Error(500, "ListAccessTokens", err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{
			ID:   tokens[i].ID,
			Name: tokens[i].Name,
			Sha1: tokens[i].Sha1,
		}
	}
	ctx.JSON(200, &apiTokens)
}

// CreateAccessToken create access tokens
func CreateAccessToken(ctx *context.APIContext, form api.CreateAccessTokenOption) {
	// swagger:operation POST /users/{username}/tokens user userCreateToken
	// ---
	// summary: Create an access token
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: accessToken
	//   in: body
	//   schema:
	//     type: object
	//     required:
	//       - name
	//     properties:
	//       name:
	//         type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/AccessToken"
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
		ID:   t.ID,
	})
}

// DeleteAccessToken delete access tokens
func DeleteAccessToken(ctx *context.APIContext) {
	// swagger:operation DELETE /users/{username}/tokens/{token} user userDeleteAccessToken
	// ---
	// summary: delete an access token
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: token
	//   in: path
	//   description: token to be deleted
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	tokenID := ctx.ParamsInt64(":id")
	if err := models.DeleteAccessTokenByID(tokenID, ctx.User.ID); err != nil {
		if models.IsErrAccessTokenNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "DeleteAccessTokenByID", err)
		}
		return
	}

	ctx.Status(204)
}
