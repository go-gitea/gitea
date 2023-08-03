// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/azuread"
	"github.com/markbates/goth/providers/bitbucket"
	"github.com/markbates/goth/providers/discord"
	"github.com/markbates/goth/providers/dropbox"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/microsoftonline"
	"github.com/markbates/goth/providers/twitter"
	"github.com/markbates/goth/providers/yandex"
)

// SimpleProviderNewFn create goth.Providers without custom url features
type SimpleProviderNewFn func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider

// SimpleProvider is a GothProvider which does not have custom url features
type SimpleProvider struct {
	BaseProvider
	scopes []string
	newFn  SimpleProviderNewFn
}

// CreateGothProvider creates a GothProvider from this Provider
func (c *SimpleProvider) CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error) {
	scopes := make([]string, len(c.scopes)+len(source.Scopes))
	copy(scopes, c.scopes)
	copy(scopes[len(c.scopes):], source.Scopes)
	return c.newFn(source.ClientID, source.ClientSecret, callbackURL, scopes...), nil
}

// NewSimpleProvider is a constructor function for simple providers
func NewSimpleProvider(name, displayName string, scopes []string, newFn SimpleProviderNewFn) *SimpleProvider {
	return &SimpleProvider{
		BaseProvider: BaseProvider{
			name:        name,
			displayName: displayName,
		},
		scopes: scopes,
		newFn:  newFn,
	}
}

var _ GothProvider = &SimpleProvider{}

func init() {
	RegisterGothProvider(
		NewSimpleProvider("bitbucket", "Bitbucket", []string{"account"},
			func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
				return bitbucket.New(clientKey, secret, callbackURL, scopes...)
			}))

	RegisterGothProvider(
		NewSimpleProvider("dropbox", "Dropbox", nil,
			func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
				return dropbox.New(clientKey, secret, callbackURL, scopes...)
			}))

	RegisterGothProvider(NewSimpleProvider("facebook", "Facebook", nil,
		func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
			return facebook.New(clientKey, secret, callbackURL, scopes...)
		}))

	// named gplus due to legacy gplus -> google migration (Google killed Google+). This ensures old connections still work
	RegisterGothProvider(NewSimpleProvider("gplus", "Google", []string{"email"},
		func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
			if setting.OAuth2Client.UpdateAvatar || setting.OAuth2Client.EnableAutoRegistration {
				scopes = append(scopes, "profile")
			}
			return google.New(clientKey, secret, callbackURL, scopes...)
		}))

	RegisterGothProvider(NewSimpleProvider("twitter", "Twitter", nil,
		func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
			return twitter.New(clientKey, secret, callbackURL)
		}))

	RegisterGothProvider(NewSimpleProvider("discord", "Discord", []string{discord.ScopeIdentify, discord.ScopeEmail},
		func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
			return discord.New(clientKey, secret, callbackURL, scopes...)
		}))

	// See https://tech.yandex.com/passport/doc/dg/reference/response-docpage/
	RegisterGothProvider(NewSimpleProvider("yandex", "Yandex", []string{"login:email", "login:info", "login:avatar"},
		func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
			return yandex.New(clientKey, secret, callbackURL, scopes...)
		}))

	RegisterGothProvider(NewSimpleProvider(
		"azuread", "Azure AD", nil,
		func(clientID, secret, callbackURL string, scopes ...string) goth.Provider {
			return azuread.New(clientID, secret, callbackURL, nil, scopes...)
		},
	))

	RegisterGothProvider(NewSimpleProvider(
		"microsoftonline", "Microsoft Online", nil,
		func(clientID, secret, callbackURL string, scopes ...string) goth.Provider {
			return microsoftonline.New(clientID, secret, callbackURL, scopes...)
		},
	))
}
