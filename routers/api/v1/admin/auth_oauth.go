// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// CreateOauthAuth create a new external authentication for oauth2
func CreateOauthAuth(ctx *context.APIContext) {
	// swagger:operation PUT /admin/identity-auth/oauth admin adminCreateOauth2Auth
	// ---
	// summary: Create an OAuth2 authentication source
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateAuthOauth2Option"
	// responses:
	//   "201":
	//     description: OAuth2 authentication source created successfully
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateAuthOauth2Option)

	discoveryURL, err := url.Parse(form.ProviderAutoDiscoveryURL)
	if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
		_ = fmt.Errorf("invalid Auto Discovery URL: %s (this must be a valid URL starting with http:// or https://)", form.ProviderAutoDiscoveryURL)
		ctx.HTTPError(http.StatusBadRequest, fmt.Sprintf("invalid Auto Discovery URL: %s (this must be a valid URL starting with http:// or https://)", form.ProviderAutoDiscoveryURL))
	}

	config := &oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      form.ProviderClientID,
		ClientSecret:                  form.ProviderClientSecret,
		OpenIDConnectAutoDiscoveryURL: form.ProviderAutoDiscoveryURL,
		CustomURLMapping:              nil,
		IconURL:                       form.ProviderIconURL,
		Scopes:                        []string{},
		RequiredClaimName:             form.RequiredClaimName,
		RequiredClaimValue:            form.RequiredClaimValue,
		SkipLocalTwoFA:                form.SkipLocal2FA,

		GroupClaimName:      form.ClaimNameProvidingGroupNameForSource,
		RestrictedGroup:     form.GroupClaimValueForRestrictedUsers,
		AdminGroup:          form.GroupClaimValueForAdministratorUsers,
		GroupTeamMap:        form.MapClaimedGroupsToOrganizationTeams,
		GroupTeamMapRemoval: form.RemoveUsersFromSyncronizedTeams,
	}

	createErr := auth_model.CreateSource(ctx, &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     form.AuthenticationName,
		IsActive: true,
		Cfg:      config,
	})

	if createErr != nil {
		ctx.APIErrorInternal(createErr)
		return
	}

	ctx.Status(http.StatusCreated)
}

// EditOauthAuth api for modifying a authentication method
func EditOauthAuth(ctx *context.APIContext) {
	// swagger:operation PATCH /admin/identity-auth/oauth/{id} admin adminEditOauth2Auth
	// ---
	// summary: Update an OAuth2 authentication source
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: authentication source ID
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateAuthOauth2Option"
	// responses:
	//   "201":
	//     description: OAuth2 authentication source updated successfully
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	oauthIDString := ctx.PathParam("id")
	oauthID, oauthIDErr := strconv.Atoi(oauthIDString)
	if oauthIDErr != nil {
		ctx.APIErrorInternal(oauthIDErr)
	}

	source, sourceErr := auth_model.GetSourceByID(ctx, int64(oauthID))
	if sourceErr != nil {
		ctx.APIErrorInternal(sourceErr)
		return
	}

	if source.Type != auth_model.OAuth2 {
		ctx.APIErrorNotFound()
		return
	}

	form := web.GetForm(ctx).(*api.EditAuthOauth2Option)

	config := &oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      form.ProviderClientID,
		ClientSecret:                  form.ProviderClientSecret,
		OpenIDConnectAutoDiscoveryURL: form.ProviderAutoDiscoveryURL,
		CustomURLMapping:              nil,
		IconURL:                       form.ProviderIconURL,
		Scopes:                        []string{},
		RequiredClaimName:             form.RequiredClaimName,
		RequiredClaimValue:            form.RequiredClaimValue,
		SkipLocalTwoFA:                form.SkipLocal2FA,

		GroupClaimName:      form.ClaimNameProvidingGroupNameForSource,
		RestrictedGroup:     form.GroupClaimValueForRestrictedUsers,
		AdminGroup:          form.GroupClaimValueForAdministratorUsers,
		GroupTeamMap:        form.MapClaimedGroupsToOrganizationTeams,
		GroupTeamMapRemoval: form.RemoveUsersFromSyncronizedTeams,
	}

	updateErr := auth_model.UpdateSource(ctx, &auth_model.Source{
		ID:       int64(oauthID),
		Type:     auth_model.OAuth2,
		Name:     form.AuthenticationName,
		IsActive: true,
		Cfg:      config,
	})

	if updateErr != nil {
		ctx.APIErrorInternal(updateErr)
		return
	}

	ctx.Status(http.StatusCreated)
}

// DeleteOauthAuth api for deleting a authentication method
func DeleteOauthAuth(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/identity-auth/oauth/{id} admin adminDeleteOauth2Auth
	// ---
	// summary: Delete an OAuth2 authentication source
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: authentication source ID
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     description: OAuth2 authentication source deleted successfully
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	oauthIDString := ctx.PathParam("id")
	oauthID, oauthIDErr := strconv.Atoi(oauthIDString)
	if oauthIDErr != nil {
		ctx.APIErrorInternal(oauthIDErr)
	}

	source, sourceErr := auth_model.GetSourceByID(ctx, int64(oauthID))
	if sourceErr != nil {
		ctx.APIErrorInternal(sourceErr)
		return
	}

	if source.Type != auth_model.OAuth2 {
		ctx.APIErrorNotFound()
		return
	}

	err := auth_model.DeleteSource(ctx, int64(oauthID))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusOK)
}

// SearchOauthAuth API for getting information of the configured authentication methods according the filter conditions
func SearchOauthAuth(ctx *context.APIContext) {
	// swagger:operation GET /admin/identity-auth/oauth admin adminSearchOauth2Auth
	// ---
	// summary: Search OAuth2 authentication sources
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
	//     description: "SearchResults of OAuth2 authentication sources"
	//     schema:
	//       type: array
	//       items:
	//         "$ref": "#/definitions/AuthOauth2Option"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	authSources, maxResults, err := db.FindAndCount[auth_model.Source](ctx, auth_model.FindSourcesOptions{
		LoginType: auth_model.OAuth2,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	results := make([]*api.AuthSourceOption, len(authSources))
	for i := range authSources {
		results[i] = convert.ToOauthProvider(ctx, authSources[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}
