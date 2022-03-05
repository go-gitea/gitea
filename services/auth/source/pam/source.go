// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/json"
)

// __________  _____      _____
// \______   \/  _  \    /     \
//  |     ___/  /_\  \  /  \ /  \
//  |    |  /    |    \/    Y    \
//  |____|  \____|__  /\____|__  /
//                  \/         \/

// Source holds configuration for the PAM login source.
type Source struct {
	ServiceName    string // pam service (e.g. system-auth)
	EmailDomain    string
	SkipLocalTwoFA bool `json:",omitempty"` // Skip Local 2fa for users authenticated with this source

	// reference to the authSource
	authSource *auth.Source
}

// FromDB fills up a PAMConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports a PAMConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

// SetAuthSource sets the related AuthSource
func (source *Source) SetAuthSource(authSource *auth.Source) {
	source.authSource = authSource
}

func init() {
	auth.RegisterTypeConfig(auth.PAM, &Source{})
}
