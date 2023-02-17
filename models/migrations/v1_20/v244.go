// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddExclusiveLabel(x *xorm.Engine) error {
	type Label struct {
		Exclusive bool
	}

	if err := x.Sync2(new(Label)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
