// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package token

import (
	"net/http"

	auth_model "gitea.dev/models/auth"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/auth/httpauth"
	api "gitea.dev/modules/structs"
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
	//	"200":
	//	  "$ref": "#/responses/CurrentAccessToken"
	accessToken := getToken(ctx)
	if accessToken == nil {
		return
	}

	// Get user info
	user, err := user_model.GetUserByID(ctx, accessToken.UID)
	if err != nil {
		ctx.APIErrorInternal(err)
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
	//	"204":
	//	  description: token deleted
	accessToken := getToken(ctx)
	if accessToken == nil {
		return
	}

	// Delete the token
	if err := auth_model.DeleteAccessTokenByID(ctx, accessToken.ID, accessToken.UID); err != nil {
		if auth_model.IsErrAccessTokenNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// getToken retrieves an access token from the API context's Authorization header and validates it against the database.
// Returns nil if the token is invalid and handles the response
func getToken(ctx *context.APIContext) *auth_model.AccessToken {
	authHeader := ctx.Req.Header.Get("Authorization")
	parsed, ok := httpauth.ParseAuthorizationHeader(authHeader)
	if !ok || parsed.BearerToken == nil {
		ctx.APIError(http.StatusNotFound, "invalid personal token")
		return nil
	}

	accessToken, err := auth_model.GetAccessTokenBySHA(ctx, parsed.BearerToken.Token)
	if err != nil {
		if auth_model.IsErrAccessTokenNotExist(err) || auth_model.IsErrAccessTokenEmpty(err) {
			ctx.APIError(http.StatusNotFound, "token not found")
			return nil
		}
		ctx.APIErrorInternal(err)
		return nil
	}
	return accessToken
}
