// Copyright 2016 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

// ExternalLoginUser makes the connecting between some existing user and additional external login sources
type ExternalLoginUser struct {
	ExternalID    string `xorm:"NOT NULL"`
	UserID        int64  `xorm:"NOT NULL"`
	LoginSourceID int64  `xorm:"NOT NULL"`
}

func addExternalLoginUser(x *xorm.Engine) error {
	if err := x.Sync2(new(ExternalLoginUser)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
