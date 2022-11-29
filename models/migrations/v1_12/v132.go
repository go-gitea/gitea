// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddBranchProtectionProtectedFilesColumn(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ProtectedFilePatterns string `xorm:"TEXT"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
