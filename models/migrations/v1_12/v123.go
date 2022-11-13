// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddReactionOriginals(x *xorm.Engine) error {
	type Reaction struct {
		OriginalAuthorID int64 `xorm:"INDEX NOT NULL DEFAULT(0)"`
		OriginalAuthor   string
	}

	return x.Sync2(new(Reaction))
}
