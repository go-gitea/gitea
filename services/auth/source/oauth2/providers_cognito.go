// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"html/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/openidConnect"
)

// CognitoProvider is a GothProvider for AWS Cognito
type CognitoProvider struct{}

func (c *CognitoProvider) SupportSSHPublicKey() bool {
	return true
}

// Name provides the technical name for this provider
func (c *CognitoProvider) Name() string {
	return "cognito"
}

// DisplayName returns the friendly name for this provider
func (c *CognitoProvider) DisplayName() string {
	return "AWS Cognito"
}

// IconHTML returns icon HTML for this provider
func (c *CognitoProvider) IconHTML(size int) template.HTML {
	return svg.RenderHTML("gitea-openid", size)
}

// CreateGothProvider creates a GothProvider from this Provider
func (c *CognitoProvider) CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error) {
	scopes := setting.OAuth2Client.OpenIDConnectScopes
	if len(scopes) == 0 {
		scopes = append(scopes, source.Scopes...)
	}

	provider, err := openidConnect.New(source.ClientID, source.ClientSecret, callbackURL, source.OpenIDConnectAutoDiscoveryURL, scopes...)
	if err != nil {
		log.Warn("Failed to create AWS Cognito Provider with name '%s' with url '%s': %v", providerName, source.OpenIDConnectAutoDiscoveryURL, err)
		return nil, err
	}
	if source.ExternalIDClaim != "" {
		// UserIdClaims is a fallback list; goth returns the first non-empty matching claim.
		// A single entry is sufficient because the admin explicitly chooses one claim (e.g. "sub" for Cognito).
		provider.UserIdClaims = []string{source.ExternalIDClaim}
	}
	return provider, nil
}

// CustomURLSettings returns the custom url settings for this provider
func (c *CognitoProvider) CustomURLSettings() *CustomURLSettings {
	return nil
}

var _ GothProvider = &CognitoProvider{}

func init() {
	RegisterGothProvider(&CognitoProvider{})
}
