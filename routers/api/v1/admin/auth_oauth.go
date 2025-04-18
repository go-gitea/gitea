// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"net/url"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// CreateOauthAuth create a new external authentication for oauth2
func CreateOauthAuth(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateAuthOauth2Option)

	var scopes []string
	// for _, s := range strings.Split(form.Oauth2Scopes, ",") {
	// 	s = strings.TrimSpace(s)
	// 	if s != "" {
	// 		scopes = append(scopes, s)
	// 	}
	// }

	discoveryURL, err := url.Parse(form.ProviderAutoDiscoveryURL)
	if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
		fmt.Errorf("invalid Auto Discovery URL: %s (this must be a valid URL starting with http:// or https://)", form.ProviderAutoDiscoveryURL)

		// todo: implement handling
	}

	config := &oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      form.ProviderClientID,
		ClientSecret:                  form.ProviderClientSecret,
		OpenIDConnectAutoDiscoveryURL: form.ProviderAutoDiscoveryURL,
		CustomURLMapping:              nil,
		IconURL:                       form.ProviderIconURL,
		Scopes:                        scopes,
		RequiredClaimName:             form.RequiredClaimName,
		RequiredClaimValue:            form.RequiredClaimValue,
		SkipLocalTwoFA:                form.SkipLocal2FA,

		GroupClaimName:      form.ClaimNameProvidingGroupNameForSource,
		RestrictedGroup:     form.GroupClaimValueForRestrictedUsers,
		AdminGroup:          form.GroupClaimValueForAdministratorUsers,
		GroupTeamMap:        form.MapClaimedGroupsToOrganizationTeams,
		GroupTeamMapRemoval: form.RemoveUsersFromSyncronizedTeams,
	}

	auth_model.CreateSource(ctx, &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     form.AuthenticationName,
		IsActive: true,
		Cfg:      config,
	})

	ctx.Status(http.StatusCreated)

	// ctx.JSON(http.StatusCreated, convert.ToUser(ctx, u, ctx.Doer))
}

// EditOauthAuth api for modifying a authentication method
func EditOauthAuth(ctx *context.APIContext) {
}

// DeleteOauthAuth api for deleting a authentication method
func DeleteOauthAuth(ctx *context.APIContext) {
}

// // SearchOauthAuth API for getting information of the configured authentication methods according the filter conditions
func SearchOauthAuth(ctx *context.APIContext) {
	listOptions := utils.GetListOptions(ctx)

	authSources, maxResults, err := db.FindAndCount[auth.Source](ctx, auth.FindSourcesOptions{})
	// fmt.Printf("Count: %d, models: %v, err: %v", count, models[0].Name, err)

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
