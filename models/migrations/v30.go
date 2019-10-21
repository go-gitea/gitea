// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addExternalLoginUserPK(x *xorm.Engine) error {
	// ExternalLoginUser see models/external_login_user.go
	type ExternalLoginUser struct {
		ExternalID    string `xorm:"pk NOT NULL"`
		UserID        int64  `xorm:"INDEX NOT NULL"`
		LoginSourceID int64  `xorm:"pk NOT NULL"`
	}

	extlogins := make([]*ExternalLoginUser, 0, 6)
	if err := x.Find(&extlogins); err != nil {
		return fmt.Errorf("Find: %v", err)
	}

	if err := x.DropTables(new(ExternalLoginUser)); err != nil {
		return fmt.Errorf("DropTables: %v", err)
	}

	if err := x.Sync2(new(ExternalLoginUser)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := x.Insert(extlogins); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}
	return nil
}
