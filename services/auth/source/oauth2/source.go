// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"gitea.dev/models/auth"
	"gitea.dev/modules/json"
)

// Source holds configuration for the OAuth2 login source.
type Source struct {
	auth.ConfigBase `json:"-"`

	Provider                      string
	ClientID                      string
	ClientSecret                  string
	OpenIDConnectAutoDiscoveryURL string
	CustomURLMapping              *CustomURLMapping
	IconURL                       string

	Scopes              []string
	RequiredClaimName   string
	RequiredClaimValue  string
	GroupClaimName      string
	AdminGroup          string
	GroupTeamMap        string
	GroupTeamMapRemoval bool
	RestrictedGroup     string

	SSHPublicKeyClaimName string
	FullNameClaimName     string
	ExternalIDClaim       string
}

func (source *Source) FromDB(bs []byte) error {
	auth.UnmarshalAuthSourceConfig(bs, source)
	return nil
}

func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

func init() {
	auth.RegisterTypeConfig(auth.OAuth2, &Source{})
}
