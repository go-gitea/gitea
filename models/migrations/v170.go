// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addDismissedReviewColumn(x *xorm.Engine) error {
	type Review struct {
		Dismissed bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Review)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
