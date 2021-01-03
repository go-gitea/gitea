// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func changeSomeColumnsLengthOfExternalLoginUser(x *xorm.Engine) error {
	type ExternalLoginUser struct {
		AccessToken       string `xorm:"TEXT"`
		AccessTokenSecret string `xorm:"TEXT"`
		RefreshToken      string `xorm:"TEXT"`
	}

	return x.Sync2(new(ExternalLoginUser))
}
