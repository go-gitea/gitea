// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
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
		ctx.Error(http.StatusInternalServerError, "ListAccessTokens", err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{
			ID:             tokens[i].ID,
			Name:           tokens[i].Name,
			TokenLastEight: tokens[i].TokenLastEight,
		}
	}
	ctx.JSON(http.StatusOK, &apiTokens)
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
		ctx.Error(http.StatusInternalServerError, "NewAccessToken", err)
		return
	}
	ctx.JSON(http.StatusCreated, &api.AccessToken{
		Name:           t.Name,
		Token:          t.Token,
		ID:             t.ID,
		TokenLastEight: t.TokenLastEight,
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
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteAccessTokenByID", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
