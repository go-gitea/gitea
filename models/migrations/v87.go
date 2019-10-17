// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addAvatarFieldToRepository(x *xorm.Engine) error {
	type Repository struct {
		// ID(10-20)-md5(32) - must fit into 64 symbols
		Avatar string `xorm:"VARCHAR(64)"`
	}

	return x.Sync2(new(Repository))
}
