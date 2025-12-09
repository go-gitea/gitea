// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"xorm.io/xorm"
)

func AddAvatarFieldToRepository(x *xorm.Engine) error {
	type Repository struct {
		// ID(10-20)-md5(32) - must fit into 64 symbols
		Avatar string `xorm:"VARCHAR(64)"`
	}

	return x.Sync(new(Repository))
}
