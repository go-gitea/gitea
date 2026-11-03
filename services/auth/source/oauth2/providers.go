// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"sort"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"

	"github.com/markbates/goth"
)

// Provider is an interface for describing a single OAuth2 provider
type Provider interface {
	Name() string
	DisplayName() string
	IconHTML(size int) template.HTML
	CustomURLSettings() *CustomURLSettings
}

// GothProviderCreator is a interface for creating a goth.Provider for a Provider
type GothProviderCreator interface {
	Provider
	CreateGothProvider(providerName, callbackURL string, source *Source) (goth.Provider, error)
}

// Providers contains the map of registered OAuth2 providers in Gitea (based on goth)
// key is used to map the OAuth2Provider with the goth provider type (also in AuthSource.OAuth2Config.Provider)
// value is used to store display data
var gothProviders = map[string]GothProviderCreator{}

// RegisterGothProvider registers a GothProviderCreator
func RegisterGothProvider(provider GothProviderCreator) {
	if _, has := gothProviders[provider.Name()]; has {
		log.Fatal("Duplicate oauth2 provider type provided: %s", provider.Name())
	}
	gothProviders[provider.Name()] = provider
}

// GetSupportedOAuth2Providers returns the slice of unconfigured OAuth2 providers
// sorted by provider name (A-Z)
func GetSupportedOAuth2Providers() []Provider {
	providers := make([]Provider, 0, len(gothProviders))

	for _, provider := range gothProviders {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name() < providers[j].Name()
	})
	return providers
}

func GetOAuth2Provider(providerName string) (Provider, error) {
	provider := gothProviders[providerName]
	if provider == nil {
		return nil, fmt.Errorf("unknown oauth2 provider: %s", providerName)
	}
	return provider, nil
}

// DefaultCallbackURL returns the default OAuth2 callback URL configured via setting.AppURL
func DefaultCallbackURL(providerName string) string {
	return setting.AppURL + "user/oauth2/" + url.PathEscape(providerName) + "/callback"
}

// RegisterProviderWithGothic registers a OAuth2 provider in goth lib
func RegisterProviderWithGothic(providerName string, source *Source) error {
	provider, err := createProvider(providerName, source)
	if err == nil && provider != nil {
		gothProvidersLock.Lock()
		defer gothProvidersLock.Unlock()

		goth.UseProviders(provider)
	}

	return err
}

// RemoveProviderFromGothic removes the given OAuth2 provider from the goth lib
func RemoveProviderFromGothic(providerName string) {
	gothProvidersLock.Lock()
	defer gothProvidersLock.Unlock()

	delete(goth.GetProviders(), providerName)
}

// ClearProviders clears all OAuth2 providers from the goth lib
func ClearProviders() {
	gothProvidersLock.Lock()
	defer gothProvidersLock.Unlock()

	goth.ClearProviders()
}

const defaultIconName = "gitea-openid"

func getIconHTML(name string, size int) template.HTML {
	svgName := "gitea-" + name
	if !slices.Contains(svg.GetIconNames(), svgName) {
		log.Debug("No SVG icon %s, use default %s", svgName, defaultIconName)
		svgName = defaultIconName
	}
	return svg.RenderHTML(svgName, size, "gt-mr-3")
}

// createProvider creates a goth.Provider using the default setting.AppURL callback URL
func createProvider(providerName string, source *Source) (goth.Provider, error) {
	return createProviderWithCallbackURL(providerName, DefaultCallbackURL(providerName), source)
}

// createProviderWithCallbackURL creates a goth.Provider using the specified callback URL
func createProviderWithCallbackURL(providerName, callbackURL string, source *Source) (goth.Provider, error) {
	providerCreator, has := gothProviders[source.Provider]
	if !has {
		return nil, fmt.Errorf("unable to find provider creator for %q (source %q)", source.Provider, providerName)
	}
	provider, err := providerCreator.CreateGothProvider(providerName, callbackURL, source)
	if err != nil {
		return nil, err
	}
	provider.SetName(providerName)
	return provider, nil
}
