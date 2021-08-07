// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/azureadv2"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/markbates/goth/providers/mastodon"
	"github.com/markbates/goth/providers/nextcloud"
)

// CustomProviderNewFn creates a goth.Provider using a custom url mapping
type CustomProviderNewFn func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error)

// CustomProvider is a GothProvider that has CustomURL features
type CustomProvider struct {
	BaseProvider
	customURLSettings *CustomURLSettings
	newFn             CustomProviderNewFn
}

// CustomURLSettings returns the CustomURLSettings for this provider
func (c *CustomProvider) CustomURLSettings() *CustomURLSettings {
	return c.customURLSettings
}

// CreateGothProvider creates a GothProvider from this Provider
func (c *CustomProvider) CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error) {
	custom := c.customURLSettings.OverrideWith(source.CustomURLMapping)

	return c.newFn(source.ClientID, source.ClientSecret, callbackURL, custom)
}

// NewCustomProvider is a constructor function for custom providers
func NewCustomProvider(name, displayName string, customURLSetting *CustomURLSettings, newFn CustomProviderNewFn) *CustomProvider {
	return &CustomProvider{
		BaseProvider: BaseProvider{
			name:        name,
			displayName: displayName,
		},
		customURLSettings: customURLSetting,
		newFn:             newFn,
	}
}

var _ (GothProvider) = &CustomProvider{}

func init() {
	RegisterGothProvider(NewCustomProvider(
		"github", "GitHub", &CustomURLSettings{
			TokenURL:   availableAttribute(gitea.TokenURL),
			AuthURL:    availableAttribute(github.AuthURL),
			ProfileURL: availableAttribute(github.ProfileURL),
			EmailURL:   availableAttribute(github.EmailURL),
		},
		func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			scopes := []string{}
			if setting.OAuth2Client.EnableAutoRegistration {
				scopes = append(scopes, "user:email")
			}
			return github.NewCustomisedURL(clientID, secret, callbackURL, custom.AuthURL, custom.TokenURL, custom.ProfileURL, custom.EmailURL, scopes...), nil
		}))

	RegisterGothProvider(NewCustomProvider(
		"gitlab", "GitLab", &CustomURLSettings{
			AuthURL:    availableAttribute(gitlab.AuthURL),
			TokenURL:   availableAttribute(gitlab.TokenURL),
			ProfileURL: availableAttribute(gitlab.ProfileURL),
		}, func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			return gitlab.NewCustomisedURL(clientID, secret, callbackURL, custom.AuthURL, custom.TokenURL, custom.ProfileURL, "read_user"), nil
		}))

	RegisterGothProvider(NewCustomProvider(
		"gitea", "Gitea", &CustomURLSettings{
			TokenURL:   requiredAttribute(gitea.TokenURL),
			AuthURL:    requiredAttribute(gitea.AuthURL),
			ProfileURL: requiredAttribute(gitea.ProfileURL),
		},
		func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			return gitea.NewCustomisedURL(clientID, secret, callbackURL, custom.AuthURL, custom.TokenURL, custom.ProfileURL), nil
		}))

	RegisterGothProvider(NewCustomProvider(
		"nextcloud", "Nextcloud", &CustomURLSettings{
			TokenURL:   requiredAttribute(nextcloud.TokenURL),
			AuthURL:    requiredAttribute(nextcloud.AuthURL),
			ProfileURL: requiredAttribute(nextcloud.ProfileURL),
		},
		func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			return nextcloud.NewCustomisedURL(clientID, secret, callbackURL, custom.AuthURL, custom.TokenURL, custom.ProfileURL), nil
		}))

	RegisterGothProvider(NewCustomProvider(
		"mastodon", "Mastodon", &CustomURLSettings{
			AuthURL: requiredAttribute(mastodon.InstanceURL),
		},
		func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			return mastodon.NewCustomisedURL(clientID, secret, callbackURL, custom.AuthURL), nil
		}))

	RegisterGothProvider(NewCustomProvider(
		"azureadv2", "Azure AD v2", &CustomURLSettings{
			Tenant: requiredAttribute("organizations"),
		},
		func(clientID, secret, callbackURL string, custom *CustomURLMapping) (goth.Provider, error) {
			return azureadv2.New(clientID, secret, callbackURL, azureadv2.ProviderOptions{
				Tenant: azureadv2.TenantType(custom.Tenant),
			}), nil
		},
	))
}
