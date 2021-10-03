// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/modules/json"
)

// ________      _____          __  .__     ________
// \_____  \    /  _  \  __ ___/  |_|  |__  \_____  \
// /   |   \  /  /_\  \|  |  \   __\  |  \  /  ____/
// /    |    \/    |    \  |  /|  | |   Y  \/       \
// \_______  /\____|__  /____/ |__| |___|  /\_______ \
//         \/         \/                 \/         \/

// Source holds configuration for the OAuth2 login source.
type Source struct {
	Provider                      string
	ClientID                      string
	ClientSecret                  string
	OpenIDConnectAutoDiscoveryURL string
	CustomURLMapping              *CustomURLMapping
	IconURL                       string
	SkipLocalTwoFA                bool `json:",omitempty"`

	// reference to the loginSource
	loginSource *login.Source
}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return models.JSONUnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports an SMTPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

// SetLoginSource sets the related LoginSource
func (source *Source) SetLoginSource(loginSource *login.Source) {
	source.loginSource = loginSource
}

func init() {
	login.RegisterTypeConfig(login.OAuth2, &Source{})
}
