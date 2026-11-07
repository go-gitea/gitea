// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func FeatureChangeTargetBranch(_ context.Context, x base.EngineMigration) error {
	type Comment struct {
		OldRef string
		NewRef string
	}

	if err := x.Sync(new(Comment)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
