// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"fmt"

	"xorm.io/xorm"
)

func AddBranchProtectionUnprotectedFilesColumn(x *xorm.Engine) error {
	type ProtectedBranch struct {
		UnprotectedFilePatterns string `xorm:"TEXT"`
	}

	if err := x.Sync(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
