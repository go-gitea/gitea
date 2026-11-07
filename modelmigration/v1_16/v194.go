// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddBranchProtectionUnprotectedFilesColumn(_ context.Context, x base.EngineMigration) error {
	type ProtectedBranch struct {
		UnprotectedFilePatterns string `xorm:"TEXT"`
	}

	if err := x.Sync(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
