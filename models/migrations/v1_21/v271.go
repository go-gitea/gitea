// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

func OAuth2ApplicationAddLockedProperty(x *xorm.Engine) error {
	type OAuth2Application struct {
		Locked bool `xorm:"NOT NULL DEFAULT FALSE"`
	}
	return x.Sync(new(OAuth2Application))
}
