// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"sort"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/util"
)

// Providers is list of known/available providers.
type Providers map[string]Source

var providers = Providers{}

// Provider is an interface for describing a single SAML provider
type Provider interface {
	Name() string
	IconHTML(size int) template.HTML
}

// AuthSourceProvider is a SAML provider
type AuthSourceProvider struct {
	sourceName, iconURL string
}

func (p *AuthSourceProvider) Name() string {
	return p.sourceName
}

func (p *AuthSourceProvider) IconHTML(size int) template.HTML {
	if p.iconURL != "" {
		return template.HTML(fmt.Sprintf(`<img class="gt-object-contain gt-mr-3" width="%d" height="%d" src="%s" alt="%s">`,
			size,
			size,
			html.EscapeString(p.iconURL), html.EscapeString(p.Name()),
		))
	}
	return svg.RenderHTML("gitea-lock-cog", size, "gt-mr-3")
}

func readIdentityProviderMetadata(ctx context.Context, source *Source) ([]byte, error) {
	if source.IdentityProviderMetadata != "" {
		return []byte(source.IdentityProviderMetadata), nil
	}

	req := httplib.NewRequest(source.IdentityProviderMetadataURL, "GET")
	req.SetTimeout(20*time.Second, time.Minute)
	resp, err := req.Response()
	if err != nil {
		return nil, fmt.Errorf("Unable to contact gitea: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func createProviderFromSource(source *auth.Source) (Provider, error) {
	samlCfg, ok := source.Cfg.(*Source)
	if !ok {
		return nil, fmt.Errorf("invalid SAML source config: %v", samlCfg)
	}
	return &AuthSourceProvider{sourceName: source.Name, iconURL: samlCfg.IconURL}, nil
}

// GetSAMLProviders returns the list of configured SAML providers
func GetSAMLProviders(ctx context.Context, isActive util.OptionalBool) ([]Provider, error) {
	authSources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{
		IsActive:  isActive,
		LoginType: auth.SAML,
	})
	if err != nil {
		return nil, err
	}

	samlProviders := make([]Provider, 0, len(authSources))
	for _, source := range authSources {
		p, err := createProviderFromSource(source)
		if err != nil {
			return nil, err
		}
		samlProviders = append(samlProviders, p)
	}

	sort.Slice(samlProviders, func(i, j int) bool {
		return samlProviders[i].Name() < samlProviders[j].Name()
	})

	return samlProviders, nil
}
