// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package smtp

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/modules/json"
)

//   _________   __________________________
//  /   _____/  /     \__    ___/\______   \
//  \_____  \  /  \ /  \|    |    |     ___/
//  /        \/    Y    \    |    |    |
// /_______  /\____|__  /____|    |____|
//         \/         \/

// Source holds configuration for the SMTP login source.
type Source struct {
	Auth           string
	Host           string
	Port           int
	AllowedDomains string `xorm:"TEXT"`
	ForceSMTPS     bool
	SkipVerify     bool
	HeloHostname   string
	DisableHelo    bool
	SkipLocalTwoFA bool `json:",omitempty"`

	// reference to the loginSource
	loginSource *login.Source
}

// FromDB fills up an SMTPConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return models.JSONUnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports an SMTPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

// IsSkipVerify returns if SkipVerify is set
func (source *Source) IsSkipVerify() bool {
	return source.SkipVerify
}

// HasTLS returns true for SMTP
func (source *Source) HasTLS() bool {
	return true
}

// UseTLS returns if TLS is set
func (source *Source) UseTLS() bool {
	return source.ForceSMTPS || source.Port == 465
}

// SetLoginSource sets the related LoginSource
func (source *Source) SetLoginSource(loginSource *login.Source) {
	source.loginSource = loginSource
}

func init() {
	login.RegisterTypeConfig(login.SMTP, &Source{})
}
