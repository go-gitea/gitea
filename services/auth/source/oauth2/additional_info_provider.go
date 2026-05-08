// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"slices"

	google_module "code.gitea.io/gitea/modules/google"

	"github.com/markbates/goth"
	go_oauth2 "golang.org/x/oauth2"
)

// AdditionalInfoProvider is implemented by OAuth2 providers that can fetch
// additional user information (such as group memberships) that is not
// included in the standard token or userinfo response.
// The provider receives the resolved goth user, and returns a modified copy
// with any extra data injected into RawData.
type AdditionalInfoProvider interface {
	FetchAdditionalInfo(ctx context.Context, user goth.User) (goth.User, error)
	FailLoginOnAdditionalInfoError() bool
}

// GetAdditionalInfoProvider returns an AdditionalInfoProvider for the given
// source if that provider supports fetching additional info, or nil if none
// applies. The returned provider is already configured with an authenticated
// HTTP client built from the access token.
func GetAdditionalInfoProvider(source *Source, gothUser *goth.User) AdditionalInfoProvider {
	switch source.Provider {
	case "gplus":
		if slices.Contains(source.Scopes, google_module.IAMScope) {
			claimName := source.GroupClaimName
			if claimName == "" {
				claimName = "groups"
			}
			oauthToken := &go_oauth2.Token{AccessToken: gothUser.AccessToken}
			// Note: we use only the access token without a refresh token.
			// This is intentional — the token is issued moments before this
			// call during the login flow and is guaranteed to be fresh.
			authenticatedClient := go_oauth2.NewClient(context.Background(), go_oauth2.StaticTokenSource(oauthToken))
			return google_module.NewClient(authenticatedClient, claimName, isGoogleGroupClaimRequiredForLoginFlow(source))
		}
	}
	return nil
}

func isGoogleGroupClaimRequiredForLoginFlow(source *Source) bool {
	groupClaimName := source.GroupClaimName
	if groupClaimName == "" {
		groupClaimName = "groups"
	}

	// Fail closed only when login itself depends on the group claim.
	//
	// Admin/restricted/team sync can preserve the user's previous state when the
	// group claim is missing, so those options intentionally stay fail-open.
	return source.RequiredClaimName == groupClaimName
}
