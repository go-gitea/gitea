// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

// Name returns the provider name of this source
func (source *Source) Name() string {
	return source.Provider
}

// DisplayName returns the display name of this source
func (source *Source) DisplayName() string {
	provider, has := gothProviders[source.Provider]
	if !has {
		return source.Provider
	}
	return provider.DisplayName()
}
