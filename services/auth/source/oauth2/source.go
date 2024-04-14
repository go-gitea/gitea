// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
)

// Source holds configuration for the OAuth2 login source.
type Source struct {
	Provider                      string
	ClientID                      string
	ClientSecret                  string
	ClientSecretEncrypt						string // Encrypted Client Secret
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
	SkipLocalTwoFA      bool `json:",omitempty"`

	// reference to the authSource
	authSource *auth.Source
}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	err := json.UnmarshalHandleDoubleEncode(bs, &source)
	if err != nil {
		return err
	}

	if source.ClientSecretEncrypt != "" {
		source.ClientSecret, err = secret.DecryptSecret(setting.SecretKey, source.ClientSecretEncrypt)
		source.ClientSecretEncrypt = ""
	}

	return err
}

// ToDB exports an OAuth2Config to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	var err error

	source.ClientSecretEncrypt, err = secret.EncryptSecret(setting.SecretKey, source.ClientSecret)
	if err != nil {
		return nil, err
	}

	source.ClientSecret = ""

	return json.Marshal(source)
}

// SetAuthSource sets the related AuthSource
func (source *Source) SetAuthSource(authSource *auth.Source) {
	source.authSource = authSource
}

func init() {
	auth.RegisterTypeConfig(auth.OAuth2, &Source{})
}
