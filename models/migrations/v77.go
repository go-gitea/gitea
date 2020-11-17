// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addUserDefaultTheme(x *xorm.Engine) error {
	type User struct {
		Theme string `xorm:"VARCHAR(30) NOT NULL DEFAULT ''"`
	}

	return x.Sync2(new(User))
}
