// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/oauth2"
	jsoniter "github.com/json-iterator/go"
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
	CustomURLMapping              *oauth2.CustomURLMapping
	IconURL                       string
}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Unmarshal(bs, source)
}

// ToDB exports an SMTPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(source)
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginOAuth2, &Source{})
}
