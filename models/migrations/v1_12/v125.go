// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"fmt"

	"xorm.io/xorm"
)

func AddReviewMigrateInfo(x *xorm.Engine) error {
	type Review struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Review)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
