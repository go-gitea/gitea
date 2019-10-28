// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

func addLoginFailuresToUser(x *xorm.Engine) error {
	type User struct {
		LastLoginFailureUnix timeutil.TimeStamp
		LoginFailures        int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(User))
}
