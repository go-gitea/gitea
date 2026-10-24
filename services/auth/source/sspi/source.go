// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sspi

import (
	"gitea.dev/models/auth"
	"gitea.dev/modules/json"
)

//   _________ ___________________.___
//  /   _____//   _____/\______   \   |
//  \_____  \ \_____  \  |     ___/   |
//  /        \/        \ |    |   |   |
// /_______  /_______  / |____|   |___|
//         \/        \/

// Source holds configuration for SSPI single sign-on.
type Source struct {
	auth.ConfigBase `json:"-"`

	AutoCreateUsers      bool
	AutoActivateUsers    bool
	StripDomainNames     bool
	SeparatorReplacement string
	DefaultLanguage      string
}

func (cfg *Source) FromDB(bs []byte) error {
	auth.UnmarshalAuthSourceConfig(bs, cfg)
	return nil
}

func (cfg *Source) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

func init() {
	auth.RegisterTypeConfig(auth.SSPI, &Source{})
}
