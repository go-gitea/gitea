// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
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
		Scopes:                        generateScopes(),
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
	oauthIDString := ctx.PathParam("id")
	oauthID, oauthIDErr := strconv.Atoi(oauthIDString)
	if oauthIDErr != nil {
		ctx.APIErrorInternal(oauthIDErr)
	}

	form := web.GetForm(ctx).(*api.CreateAuthOauth2Option)

	config := &oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      form.ProviderClientID,
		ClientSecret:                  form.ProviderClientSecret,
		OpenIDConnectAutoDiscoveryURL: form.ProviderAutoDiscoveryURL,
		CustomURLMapping:              nil,
		IconURL:                       form.ProviderIconURL,
		Scopes:                        generateScopes(),
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

// // SearchOauthAuth API for getting information of the configured authentication methods according the filter conditions
func SearchOauthAuth(ctx *context.APIContext) {
	listOptions := utils.GetListOptions(ctx)

	authSources, maxResults, err := db.FindAndCount[auth_model.Source](ctx, auth_model.FindSourcesOptions{})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	results := make([]*api.AuthOauth2Option, len(authSources))
	for i := range authSources {
		results[i] = convert.ToOauthProvider(ctx, authSources[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}

// ??? todo: what should I do here?
func generateScopes() []string {
	var scopes []string

	// for _, s := range strings.Split(form.Oauth2Scopes, ",") {
	// 	s = strings.TrimSpace(s)
	// 	if s != "" {
	// 		scopes = append(scopes, s)
	// 	}
	// }

	return scopes
}
