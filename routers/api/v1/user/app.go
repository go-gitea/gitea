// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/AccessTokenList"

	opts := auth_model.ListAccessTokensOptions{UserID: ctx.Doer.ID, ListOptions: utils.GetListOptions(ctx)}

	count, err := auth_model.CountAccessTokens(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	tokens, err := auth_model.ListAccessTokens(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{
			ID:             tokens[i].ID,
			Name:           tokens[i].Name,
			TokenLastEight: tokens[i].TokenLastEight,
			Scopes:         tokens[i].Scope.StringSlice(),
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiTokens)
}

// CreateAccessToken create access tokens
func CreateAccessToken(ctx *context.APIContext) {
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
	//   required: true
	//   type: string
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateAccessTokenOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/AccessToken"
	//   "400":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.CreateAccessTokenOption)

	t := &auth_model.AccessToken{
		UID:  ctx.Doer.ID,
		Name: form.Name,
	}

	exist, err := auth_model.AccessTokenByNameExists(t)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if exist {
		ctx.Error(http.StatusBadRequest, "AccessTokenByNameExists", errors.New("access token name has been used already"))
		return
	}

	scope, err := auth_model.AccessTokenScope(strings.Join(form.Scopes, ",")).Normalize()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "AccessTokenScope.Normalize", fmt.Errorf("invalid access token scope provided: %w", err))
		return
	}
	t.Scope = scope

	if err := auth_model.NewAccessToken(t); err != nil {
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
	//   description: token to be deleted, identified by ID and if not available by name
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"

	token := ctx.Params(":id")
	tokenID, _ := strconv.ParseInt(token, 0, 64)

	if tokenID == 0 {
		tokens, err := auth_model.ListAccessTokens(auth_model.ListAccessTokensOptions{
			Name:   token,
			UserID: ctx.Doer.ID,
		})
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ListAccessTokens", err)
			return
		}

		switch len(tokens) {
		case 0:
			ctx.NotFound()
			return
		case 1:
			tokenID = tokens[0].ID
		default:
			ctx.Error(http.StatusUnprocessableEntity, "DeleteAccessTokenByID", fmt.Errorf("multiple matches for token name '%s'", token))
			return
		}
	}
	if tokenID == 0 {
		ctx.Error(http.StatusInternalServerError, "Invalid TokenID", nil)
		return
	}

	if err := auth_model.DeleteAccessTokenByID(tokenID, ctx.Doer.ID); err != nil {
		if auth_model.IsErrAccessTokenNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteAccessTokenByID", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateOauth2Application is the handler to create a new OAuth2 Application for the authenticated user
func CreateOauth2Application(ctx *context.APIContext) {
	// swagger:operation POST /user/applications/oauth2 user userCreateOAuth2Application
	// ---
	// summary: creates a new OAuth2 application
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateOAuth2ApplicationOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/OAuth2Application"
	//   "400":
	//     "$ref": "#/responses/error"

	data := web.GetForm(ctx).(*api.CreateOAuth2ApplicationOptions)

	app, err := auth_model.CreateOAuth2Application(ctx, auth_model.CreateOAuth2ApplicationOptions{
		Name:               data.Name,
		UserID:             ctx.Doer.ID,
		RedirectURIs:       data.RedirectURIs,
		ConfidentialClient: data.ConfidentialClient,
	})
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", "error creating oauth2 application")
		return
	}
	secret, err := app.GenerateClientSecret()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", "error creating application secret")
		return
	}
	app.ClientSecret = secret

	ctx.JSON(http.StatusCreated, convert.ToOAuth2Application(app))
}

// ListOauth2Applications list all the Oauth2 application
func ListOauth2Applications(ctx *context.APIContext) {
	// swagger:operation GET /user/applications/oauth2 user userGetOauth2Application
	// ---
	// summary: List the authenticated user's oauth2 applications
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/OAuth2ApplicationList"

	apps, total, err := auth_model.ListOAuth2Applications(ctx.Doer.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListOAuth2Applications", err)
		return
	}

	apiApps := make([]*api.OAuth2Application, len(apps))
	for i := range apps {
		apiApps[i] = convert.ToOAuth2Application(apps[i])
		apiApps[i].ClientSecret = "" // Hide secret on application list
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &apiApps)
}

// DeleteOauth2Application delete OAuth2 Application
func DeleteOauth2Application(ctx *context.APIContext) {
	// swagger:operation DELETE /user/applications/oauth2/{id} user userDeleteOAuth2Application
	// ---
	// summary: delete an OAuth2 Application
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: token to be deleted
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appID := ctx.ParamsInt64(":id")
	if err := auth_model.DeleteOAuth2Application(appID, ctx.Doer.ID); err != nil {
		if auth_model.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteOauth2ApplicationByID", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetOauth2Application get OAuth2 Application
func GetOauth2Application(ctx *context.APIContext) {
	// swagger:operation GET /user/applications/oauth2/{id} user userGetOAuth2Application
	// ---
	// summary: get an OAuth2 Application
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: Application ID to be found
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/OAuth2Application"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appID := ctx.ParamsInt64(":id")
	app, err := auth_model.GetOAuth2ApplicationByID(ctx, appID)
	if err != nil {
		if auth_model.IsErrOauthClientIDInvalid(err) || auth_model.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetOauth2ApplicationByID", err)
		}
		return
	}
	if app.UID != ctx.Doer.ID {
		ctx.NotFound()
		return
	}

	app.ClientSecret = ""

	ctx.JSON(http.StatusOK, convert.ToOAuth2Application(app))
}

// UpdateOauth2Application update OAuth2 Application
func UpdateOauth2Application(ctx *context.APIContext) {
	// swagger:operation PATCH /user/applications/oauth2/{id} user userUpdateOAuth2Application
	// ---
	// summary: update an OAuth2 Application, this includes regenerating the client secret
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: application to be updated
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateOAuth2ApplicationOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/OAuth2Application"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appID := ctx.ParamsInt64(":id")

	data := web.GetForm(ctx).(*api.CreateOAuth2ApplicationOptions)

	app, err := auth_model.UpdateOAuth2Application(auth_model.UpdateOAuth2ApplicationOptions{
		Name:               data.Name,
		UserID:             ctx.Doer.ID,
		ID:                 appID,
		RedirectURIs:       data.RedirectURIs,
		ConfidentialClient: data.ConfidentialClient,
	})
	if err != nil {
		if auth_model.IsErrOauthClientIDInvalid(err) || auth_model.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateOauth2ApplicationByID", err)
		}
		return
	}
	app.ClientSecret, err = app.GenerateClientSecret()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", "error updating application secret")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToOAuth2Application(app))
}
