// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func fixProtectedBranchCanPushValue(x *xorm.Engine) error {
	type ProtectedBranch struct {
		CanPush bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	_, err := x.Cols("can_push").Update(&ProtectedBranch{
		CanPush: false,
	})
	return err
}
