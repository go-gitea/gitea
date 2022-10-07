// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

// addConfidentialColumnToOAuth2ApplicationTable: add Confidential column, setting existing rows to true
func addConfidentialColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	type OAuth2Application struct {
		ID           int64 `xorm:"pk autoincr"`
		UID          int64 `xorm:"INDEX"`
		Name         string
		ClientID     string `xorm:"unique"`
		ClientSecret string
		Confidential bool               `xorm:"NOT NULL DEFAULT TRUE"`
		RedirectURIs []string           `xorm:"redirect_uris JSON TEXT"`
		CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix  timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(OAuth2Application))
}
