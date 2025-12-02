// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import (
	"xorm.io/xorm"
)

func ChangeSomeColumnsLengthOfExternalLoginUser(x *xorm.Engine) error {
	type ExternalLoginUser struct {
		AccessToken       string `xorm:"TEXT"`
		AccessTokenSecret string `xorm:"TEXT"`
		RefreshToken      string `xorm:"TEXT"`
	}

	return x.Sync(new(ExternalLoginUser))
}
