// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func FeatureChangeTargetBranch(x *xorm.Engine) error {
	type Comment struct {
		OldRef string
		NewRef string
	}

	if err := x.Sync(new(Comment)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
