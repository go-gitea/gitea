// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package token

import (
	"errors"
	"net/http"

	auth_model "gitea.dev/models/auth"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/auth/httpauth"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

// GetCurrentToken returns metadata about the currently authenticated token.
func GetCurrentToken(ctx *context.APIContext) {
	// swagger:operation GET /token miscellaneous getCurrentToken
	// ---
	// summary: Get the currently authenticated token
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CurrentAccessToken"
	accessToken, err := getToken(ctx)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	// Get user info
	user, err := user_model.GetUserByID(ctx, accessToken.UID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	ctx.JSON(http.StatusOK, &api.CurrentAccessToken{
		ID:         accessToken.ID,
		Name:       accessToken.Name,
		Scopes:     accessToken.Scope.StringSlice(),
		CreatedAt:  accessToken.CreatedUnix.AsTime(),
		LastUsedAt: accessToken.UpdatedUnix.AsTime(),
		User: &api.UserMeta{
			ID:    user.ID,
			Login: user.Name,
		},
	})
}

// DeleteCurrentToken deletes the currently authenticated token.
func DeleteCurrentToken(ctx *context.APIContext) {
	// swagger:operation DELETE /token miscellaneous deleteCurrentToken
	// ---
	// summary: Delete the currently authenticated token
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     description: token deleted
	accessToken, err := getToken(ctx)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	// Delete the token
	err = auth_model.DeleteAccessTokenByID(ctx, accessToken.ID, accessToken.UID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// getToken retrieves an access token from the API context's Authorization header and validates it against the database.
// Returns nil if the token is invalid and handles the response
func getToken(ctx *context.APIContext) (*auth_model.AccessToken, error) {
	authHeader := ctx.Req.Header.Get("Authorization")
	parsed, ok := httpauth.ParseAuthorizationHeader(authHeader)
	if !ok || parsed.BearerToken == nil {
		return nil, util.NewNotExistErrorf("invalid access token")
	}
	return auth_model.GetAccessTokenBySHA(ctx, parsed.BearerToken.Token)
}
