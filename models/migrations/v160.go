// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addUserPinnedRepoTable(x *xorm.Engine) error {

	type UserPinnedRepo struct {
		UID    int64 `xorm:"pk INDEX NOT NULL"`
		RepoID int64 `xorm:"pk NOT NULL"`
	}

	return x.Sync2(new(UserPinnedRepo))
}
