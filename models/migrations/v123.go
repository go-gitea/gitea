// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addReactionOriginals(x *xorm.Engine) error {
	type Reaction struct {
		OriginalAuthorID int64 `xorm:"INDEX NOT NULL DEFAULT(0)"`
		OriginalAuthor   string
	}

	return x.Sync2(new(Reaction))
}
