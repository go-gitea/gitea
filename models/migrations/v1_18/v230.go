// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18 //nolint

import (
	"xorm.io/xorm"
)

// AddConfidentialColumnToOAuth2ApplicationTable: add ConfidentialClient column, setting existing rows to true
func AddConfidentialClientColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	type OAuth2Application struct {
		ID                 int64
		ConfidentialClient bool `xorm:"NOT NULL DEFAULT TRUE"`
	}

	return x.Table("oauth2_application").Sync(new(OAuth2Application))
}
