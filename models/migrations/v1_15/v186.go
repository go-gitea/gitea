// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_15 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateProtectedTagTable(x *xorm.Engine) error {
	type ProtectedTag struct {
		ID               int64 `xorm:"pk autoincr"`
		RepoID           int64
		NamePattern      string
		AllowlistUserIDs []int64 `xorm:"JSON TEXT"`
		AllowlistTeamIDs []int64 `xorm:"JSON TEXT"`

		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync2(new(ProtectedTag))
}
