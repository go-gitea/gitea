// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addUserRedirect(x *xorm.Engine) (err error) {
	type UserRedirect struct {
		ID             int64  `xorm:"pk autoincr"`
		LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		RedirectUserID int64
	}

	if err := x.Sync2(new(UserRedirect)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
