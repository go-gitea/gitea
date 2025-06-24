// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddDismissedReviewColumn(x *xorm.Engine) error {
	type Review struct {
		Dismissed bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Review)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
