// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sspi

import (
	"gitea.dev/models/auth"
	"gitea.dev/modules/json"
)

// Source holds configuration for SSPI single sign-on.
type Source struct {
	auth.ConfigBase `json:"-"`

	AutoCreateUsers      bool
	AutoActivateUsers    bool
	StripDomainNames     bool
	SeparatorReplacement string
	DefaultLanguage      string
}

func (source *Source) FromDB(bs []byte) error {
	auth.UnmarshalAuthSourceConfig(bs, source)
	return nil
}

func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

func init() {
	auth.RegisterTypeConfig(auth.SSPI, &Source{})
}
