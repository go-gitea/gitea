// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_13 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddKeepActivityPrivateUserColumn(x *xorm.Engine) error {
	type User struct {
		KeepActivityPrivate bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(User)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
