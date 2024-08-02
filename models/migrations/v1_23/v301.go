// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

// AddSkipSeconderyAuthToOAuth2ApplicationTable: add SkipSecondaryAuthorization column, setting existing rows to false
func AddSkipSecondaryAuthColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	type oauth2Application struct {
		SkipSecondaryAuthorization bool `xorm:"NOT NULL DEFAULT FALSE"`
	}
	return x.Sync(new(oauth2Application))
}
