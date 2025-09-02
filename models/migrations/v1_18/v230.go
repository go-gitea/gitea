// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"xorm.io/xorm"
)

// AddConfidentialColumnToOAuth2ApplicationTable: add ConfidentialClient column, setting existing rows to true
func AddConfidentialClientColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	type oauth2Application struct {
		ID                 int64
		ConfidentialClient bool `xorm:"NOT NULL DEFAULT TRUE"`
	}
	return x.Sync(new(oauth2Application))
}
