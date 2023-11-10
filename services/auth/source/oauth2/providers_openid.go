// Copyright 2021 The Gitea Authors. All rights reserved.
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

// OpenIDProvider is a GothProvider for OpenID
type OpenIDProvider struct{}

// Name provides the technical name for this provider
func (o *OpenIDProvider) Name() string {
	return "openidConnect"
}

// DisplayName returns the friendly name for this provider
func (o *OpenIDProvider) DisplayName() string {
	return "OpenID Connect"
}

// IconHTML returns icon HTML for this provider
func (o *OpenIDProvider) IconHTML(size int) template.HTML {
	return svg.RenderHTML("gitea-openid", size, "gt-mr-3")
}

// CreateGothProvider creates a GothProvider from this Provider
func (o *OpenIDProvider) CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error) {
	scopes := setting.OAuth2Client.OpenIDConnectScopes
	if len(scopes) == 0 {
		scopes = append(scopes, source.Scopes...)
	}

	provider, err := openidConnect.New(source.ClientID, source.ClientSecret, callbackURL, source.OpenIDConnectAutoDiscoveryURL, scopes...)
	if err != nil {
		log.Warn("Failed to create OpenID Connect Provider with name '%s' with url '%s': %v", providerName, source.OpenIDConnectAutoDiscoveryURL, err)
	}
	return provider, err
}

// CustomURLSettings returns the custom url settings for this provider
func (o *OpenIDProvider) CustomURLSettings() *CustomURLSettings {
	return nil
}

var _ GothProvider = &OpenIDProvider{}

func init() {
	RegisterGothProvider(&OpenIDProvider{})
}
