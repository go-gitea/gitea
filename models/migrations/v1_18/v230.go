// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18 //nolint

import (
	"xorm.io/xorm"
)

type oAuth2Application struct {
	ID                 int64
	ConfidentialClient bool `xorm:"NOT NULL DEFAULT TRUE"`
}

func (oAuth2Application) TableName() string {
	return "oauth2_application"
}

// AddConfidentialColumnToOAuth2ApplicationTable: add ConfidentialClient column, setting existing rows to true
func AddConfidentialClientColumnToOAuth2ApplicationTable(x *xorm.Engine) error {
	return x.Sync(new(oAuth2Application))
}
