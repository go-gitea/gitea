// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddReactionOriginals(x *xorm.Engine) error {
	type Reaction struct {
		OriginalAuthorID int64 `xorm:"INDEX NOT NULL DEFAULT(0)"`
		OriginalAuthor   string
	}

	return x.Sync(new(Reaction))
}
