// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func changeDefaultPasswordHashingAlgorithmToArgon(x *xorm.Engine) error {
	type User struct {
		PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
	}

	return x.Sync(new(User))
}
