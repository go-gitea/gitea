// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sspi

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/json"
)

//   _________ ___________________.___
//  /   _____//   _____/\______   \   |
//  \_____  \ \_____  \  |     ___/   |
//  /        \/        \ |    |   |   |
// /_______  /_______  / |____|   |___|
//         \/        \/

// Source holds configuration for SSPI single sign-on.
type Source struct {
	AutoCreateUsers      bool
	AutoActivateUsers    bool
	StripDomainNames     bool
	SeparatorReplacement string
	DefaultLanguage      string
}

// FromDB fills up an SSPIConfig from serialized format.
func (cfg *Source) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports an SSPIConfig to a serialized format.
func (cfg *Source) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

func init() {
	auth.RegisterTypeConfig(auth.SSPI, &Source{})
}
