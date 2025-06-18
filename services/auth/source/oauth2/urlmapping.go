// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

// CustomURLMapping describes the urls values to use when customizing OAuth2 provider URLs
type CustomURLMapping struct {
	AuthURL    string `json:",omitempty"`
	TokenURL   string `json:",omitempty"`
	ProfileURL string `json:",omitempty"`
	EmailURL   string `json:",omitempty"`
	Tenant     string `json:",omitempty"`
}

// CustomURLSettings describes the urls values and availability to use when customizing OAuth2 provider URLs
type CustomURLSettings struct {
	AuthURL    Attribute
	TokenURL   Attribute
	ProfileURL Attribute
	EmailURL   Attribute
	Tenant     Attribute
}

// Attribute describes the availability, and required status for a custom url configuration
type Attribute struct {
	Value     string
	Available bool
	Required  bool
}

func availableAttribute(value string) Attribute {
	return Attribute{Value: value, Available: true}
}

func requiredAttribute(value string) Attribute {
	return Attribute{Value: value, Available: true, Required: true}
}

// Required is true if any attribute is required
func (c *CustomURLSettings) Required() bool {
	if c == nil {
		return false
	}
	if c.AuthURL.Required || c.EmailURL.Required || c.ProfileURL.Required || c.TokenURL.Required || c.Tenant.Required {
		return true
	}
	return false
}

// OverrideWith copies the current customURLMapping and overrides it with values from the provided mapping
func (c *CustomURLSettings) OverrideWith(override *CustomURLMapping) *CustomURLMapping {
	custom := &CustomURLMapping{
		AuthURL:    c.AuthURL.Value,
		TokenURL:   c.TokenURL.Value,
		ProfileURL: c.ProfileURL.Value,
		EmailURL:   c.EmailURL.Value,
		Tenant:     c.Tenant.Value,
	}
	if override != nil {
		if len(override.AuthURL) > 0 && c.AuthURL.Available {
			custom.AuthURL = override.AuthURL
		}
		if len(override.TokenURL) > 0 && c.TokenURL.Available {
			custom.TokenURL = override.TokenURL
		}
		if len(override.ProfileURL) > 0 && c.ProfileURL.Available {
			custom.ProfileURL = override.ProfileURL
		}
		if len(override.EmailURL) > 0 && c.EmailURL.Available {
			custom.EmailURL = override.EmailURL
		}
		if len(override.Tenant) > 0 && c.Tenant.Available {
			custom.Tenant = override.Tenant
		}
	}
	return custom
}
