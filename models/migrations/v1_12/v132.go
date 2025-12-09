// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"fmt"

	"xorm.io/xorm"
)

func AddBranchProtectionProtectedFilesColumn(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ProtectedFilePatterns string `xorm:"TEXT"`
	}

	if err := x.Sync(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
