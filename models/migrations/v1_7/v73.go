// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_7 // nolint

import (
	"xorm.io/xorm"
)

func AddMustChangePassword(x *xorm.Engine) error {
	// User see models/user.go
	type User struct {
		ID                 int64 `xorm:"pk autoincr"`
		MustChangePassword bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync2(new(User))
}
