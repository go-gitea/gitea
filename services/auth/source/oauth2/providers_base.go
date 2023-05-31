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
func (b *BaseProvider) Image() string {
	return "/assets/img/auth/" + b.name + ".png"
}

// Image returns svg name for this provider
func (b *BaseProvider) SvgName() string {
	if b.name == "github" {
		return "octicon-mark-github"
	}
	if b.name == "gplus" {
		return "gitea-google"
	}
	return "gitea-" + b.name
}

// CustomURLSettings returns the custom url settings for this provider
func (b *BaseProvider) CustomURLSettings() *CustomURLSettings {
	return nil
}

var _ (Provider) = &BaseProvider{}
