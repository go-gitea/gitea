// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"errors"
	"net/url"
	"sort"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth"
)

// Provider is an interface for describing a single OAuth2 provider
type Provider interface {
	Name() string
	DisplayName() string
	Image() string
	CustomURLSettings() *CustomURLSettings
}

// GothProviderCreator provides a function to create a goth.Provider
type GothProviderCreator interface {
	CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error)
}

// GothProvider is an interface for describing a single OAuth2 provider
type GothProvider interface {
	Provider
	GothProviderCreator
}

// ImagedProvider provide an overridden image setting for the provider
type ImagedProvider struct {
	GothProvider
	image string
}

// Image returns the image path for this provider
func (i *ImagedProvider) Image() string {
	return i.image
}

// NewImagedProvider is a constructor function for the ImagedProvider
func NewImagedProvider(image string, provider GothProvider) *ImagedProvider {
	return &ImagedProvider{
		GothProvider: provider,
		image:        image,
	}
}

// Providers contains the map of registered OAuth2 providers in Gitea (based on goth)
// key is used to map the OAuth2Provider with the goth provider type (also in AuthSource.OAuth2Config.Provider)
// value is used to store display data
var gothProviders = map[string]GothProvider{}

// RegisterGothProvider registers a GothProvider
func RegisterGothProvider(provider GothProvider) {
	if _, has := gothProviders[provider.Name()]; has {
		log.Fatal("Duplicate oauth2provider type provided: %s", provider.Name())
	}
	gothProviders[provider.Name()] = provider
}

// GetOAuth2Providers returns the map of unconfigured OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
func GetOAuth2Providers() []Provider {
	providers := make([]Provider, 0, len(gothProviders))

	for _, provider := range gothProviders {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name() < providers[j].Name()
	})
	return providers
}

// GetActiveOAuth2Providers returns the map of configured active OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
func GetActiveOAuth2Providers() ([]string, map[string]Provider, error) {
	// Maybe also separate used and unused providers so we can force the registration of only 1 active provider for each type

	authSources, err := auth.GetActiveOAuth2ProviderSources()
	if err != nil {
		return nil, nil, err
	}

	var orderedKeys []string
	providers := make(map[string]Provider)
	for _, source := range authSources {
		prov := gothProviders[source.Cfg.(*Source).Provider]
		if source.Cfg.(*Source).IconURL != "" {
			prov = &ImagedProvider{prov, source.Cfg.(*Source).IconURL}
		}
		providers[source.Name] = prov
		orderedKeys = append(orderedKeys, source.Name)
	}

	sort.Strings(orderedKeys)

	return orderedKeys, providers, nil
}

// RegisterProviderWithGothic register a OAuth2 provider in goth lib
func RegisterProviderWithGothic(providerName string, source *Source) error {
	provider, err := createProvider(providerName, source)

	if err == nil && provider != nil {
		gothRWMutex.Lock()
		defer gothRWMutex.Unlock()

		goth.UseProviders(provider)
	}

	return err
}

// RemoveProviderFromGothic removes the given OAuth2 provider from the goth lib
func RemoveProviderFromGothic(providerName string) {
	gothRWMutex.Lock()
	defer gothRWMutex.Unlock()

	delete(goth.GetProviders(), providerName)
}

// ClearProviders clears all OAuth2 providers from the goth lib
func ClearProviders() {
	gothRWMutex.Lock()
	defer gothRWMutex.Unlock()

	goth.ClearProviders()
}

// ErrAuthSourceNotActived login source is not actived error
var ErrAuthSourceNotActived = errors.New("auth source is not actived")

// used to create different types of goth providers
func createProvider(providerName string, source *Source) (goth.Provider, error) {
	callbackURL := setting.AppURL + "user/oauth2/" + url.PathEscape(providerName) + "/callback"

	var provider goth.Provider
	var err error

	p, ok := gothProviders[source.Provider]
	if !ok {
		return nil, ErrAuthSourceNotActived
	}

	provider, err = p.CreateGothProvider(providerName, callbackURL, source)
	if err != nil {
		return provider, err
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if err == nil && provider != nil {
		provider.SetName(providerName)
	}

	return provider, err
}
