// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"sort"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth"
)

// Provider is an interface for describing a single OAuth2 provider
type Provider interface {
	Name() string
	DisplayName() string
	IconHTML(size int) template.HTML
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

// AuthSourceProvider provides a provider for an AuthSource. Multiple auth sources could use the same registered GothProvider
// So each auth source should have its own DisplayName and IconHTML for display.
// The Name is the GothProvider's name, to help to find the GothProvider to sign in.
// The DisplayName is the auth source config's name, site admin set it on the admin page, the IconURL can also be set there.
type AuthSourceProvider struct {
	GothProvider
	sourceName, iconURL string
}

func (p *AuthSourceProvider) Name() string {
	return p.GothProvider.Name()
}

func (p *AuthSourceProvider) DisplayName() string {
	return p.sourceName
}

func (p *AuthSourceProvider) IconHTML(size int) template.HTML {
	if p.iconURL != "" {
		img := fmt.Sprintf(`<img class="tw-object-contain tw-mr-2" width="%d" height="%d" src="%s" alt="%s">`,
			size,
			size,
			html.EscapeString(p.iconURL), html.EscapeString(p.DisplayName()),
		)
		return template.HTML(img)
	}
	return p.GothProvider.IconHTML(size)
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

// GetSupportedOAuth2Providers returns the map of unconfigured OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
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

func CreateProviderFromSource(source *auth.Source) (Provider, error) {
	oauth2Cfg, ok := source.Cfg.(*Source)
	if !ok {
		return nil, fmt.Errorf("invalid OAuth2 source config: %v", oauth2Cfg)
	}
	gothProv := gothProviders[oauth2Cfg.Provider]
	return &AuthSourceProvider{GothProvider: gothProv, sourceName: source.Name, iconURL: oauth2Cfg.IconURL}, nil
}

// GetOAuth2Providers returns the list of configured OAuth2 providers
func GetOAuth2Providers(ctx context.Context, isActive optional.Option[bool]) ([]Provider, error) {
	authSources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{
		IsActive:  isActive,
		LoginType: auth.OAuth2,
	})
	if err != nil {
		return nil, err
	}

	providers := make([]Provider, 0, len(authSources))
	for _, source := range authSources {
		provider, err := CreateProviderFromSource(source)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name() < providers[j].Name()
	})

	return providers, nil
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

var ErrAuthSourceNotActivated = errors.New("auth source is not activated")

// used to create different types of goth providers
func createProvider(providerName string, source *Source) (goth.Provider, error) {
	callbackURL := setting.AppURL + "user/oauth2/" + url.PathEscape(providerName) + "/callback"

	var provider goth.Provider
	var err error

	p, ok := gothProviders[source.Provider]
	if !ok {
		return nil, ErrAuthSourceNotActivated
	}

	provider, err = p.CreateGothProvider(providerName, callbackURL, source)
	if err != nil {
		return provider, err
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if provider != nil {
		provider.SetName(providerName)
	}

	return provider, err
}
