// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddBranchProtectionUnprotectedFilesColumn(x *xorm.Engine) error {
	type ProtectedBranch struct {
		UnprotectedFilePatterns string `xorm:"TEXT"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
