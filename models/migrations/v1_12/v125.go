// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddReviewMigrateInfo(x *xorm.Engine) error {
	type Review struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync2(new(Review)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
