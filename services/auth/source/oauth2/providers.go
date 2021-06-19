// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"sort"

	"code.gitea.io/gitea/models"
)

// GetActiveOAuth2Providers returns the map of configured active OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
func GetActiveOAuth2Providers() ([]string, map[string]models.OAuth2Provider, error) {
	// Maybe also separate used and unused providers so we can force the registration of only 1 active provider for each type

	loginSources, err := models.GetActiveOAuth2ProviderLoginSources()
	if err != nil {
		return nil, nil, err
	}

	var orderedKeys []string
	providers := make(map[string]models.OAuth2Provider)
	for _, source := range loginSources {
		prov := models.OAuth2Providers[source.Cfg.(*Source).Provider]
		if source.Cfg.(*Source).IconURL != "" {
			prov.Image = source.Cfg.(*Source).IconURL
		}
		providers[source.Name] = prov
		orderedKeys = append(orderedKeys, source.Name)
	}

	sort.Strings(orderedKeys)

	return orderedKeys, providers, nil
}
