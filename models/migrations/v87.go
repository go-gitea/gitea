// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"github.com/go-xorm/xorm"
)

func addAvatarFieldToRepository(x *xorm.Engine) error {
	type Repository struct {
		Avatar string `xorm:"VARCHAR(2048)"`
	}

	return x.Sync2(new(Repository))
}
