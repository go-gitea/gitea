// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"code.gitea.io/gitea/modules/util"
)

// BaseProvider represents a common base for Provider
type BaseProvider struct {
	name        string
	displayName string
}

// Name provides the technical name for this provider
func (b *BaseProvider) Name() string {
	return b.name
}

// DisplayName returns the friendly name for this provider
func (b *BaseProvider) DisplayName() string {
	return b.displayName
}

// Image returns an image path for this provider
func (b *BaseProvider) Image() string {
	suffix := ".png"
	name := b.name
	// names of providers that have svg as their default images
	// providers_openid has its own Image() function
	defaultSvgProviderNames := []string{"azuread", "azureadv2", "bitbucket", "discord", "dropbox", "facebook", "gitea", "github", "gitlab", "gplus", "mastodon", "microsoftonline", "nextcloud", "twitter", "yandex"}
	if util.SliceContainsString(defaultSvgProviderNames, b.name) {
		suffix = ".svg"
		if b.name == "gplus" {
			name = "google"
		}
	}
	return "/assets/img/auth/" + name + suffix
}

// CustomURLSettings returns the custom url settings for this provider
func (b *BaseProvider) CustomURLSettings() *CustomURLSettings {
	return nil
}

var _ (Provider) = &BaseProvider{}
