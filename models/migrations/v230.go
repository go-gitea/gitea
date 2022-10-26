// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

// addConfidentialColumnToOAuth2ApplicationTable: add ConfidentialClient column, setting existing rows to true
func addConfidentialClientColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	type OAuth2Application struct {
		ConfidentialClient bool `xorm:"NOT NULL DEFAULT TRUE"`
	}

	return x.Sync(new(OAuth2Application))
}
