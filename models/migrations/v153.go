// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addTeamReviewRequestSupport(x *xorm.Engine) error {
	type Review struct {
		ReviewerTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type Comment struct {
		AssigneeTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(Review)); err != nil {
		return err
	}

	return x.Sync2(new(Comment))
}
