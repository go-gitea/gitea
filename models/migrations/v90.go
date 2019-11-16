// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func changeSomeColumnsLengthOfRepo(x *xorm.Engine) error {
	type Repository struct {
		ID          int64  `xorm:"pk autoincr"`
		Description string `xorm:"TEXT"`
		Website     string `xorm:"VARCHAR(2048)"`
		OriginalURL string `xorm:"VARCHAR(2048)"`
	}

	return x.Sync2(new(Repository))
}
