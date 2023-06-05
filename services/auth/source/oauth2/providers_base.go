// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

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
// Use svg for default image icons
func (b *BaseProvider) Image() string {
	name := b.name
	if b.name == "gplus" {
		name = "google"
	}
	return "/assets/img/auth/" + name + ".svg"
}

// CustomURLSettings returns the custom url settings for this provider
func (b *BaseProvider) CustomURLSettings() *CustomURLSettings {
	return nil
}

var _ (Provider) = &BaseProvider{}
