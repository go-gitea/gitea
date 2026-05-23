// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package smtp

import (
	"code.gitea.io/gitea/models/auth"
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
	auth.ConfigBase `json:"-"`

	Auth           string
	Host           string
	Port           int
	AllowedDomains string `xorm:"TEXT"`
	ForceSMTPS     bool
	SkipVerify     bool
	HeloHostname   string
	DisableHelo    bool
}

// FromDB fills up an SMTPConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &source)
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

func init() {
	auth.RegisterTypeConfig(auth.SMTP, &Source{})
}
