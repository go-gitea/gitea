// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addSizeLimitOnRepo(x *xorm.Engine) error {
	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		SizeLimit int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(Repository))
}
