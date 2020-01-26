// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

// UserOpenID is the list of all OpenID identities of a user.
type UserOpenID struct {
	ID  int64  `xorm:"pk autoincr"`
	UID int64  `xorm:"INDEX NOT NULL"`
	URI string `xorm:"UNIQUE NOT NULL"`
}

func addUserOpenID(x *xorm.Engine) error {
	if err := x.Sync2(new(UserOpenID)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
