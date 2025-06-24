// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddOrgIDLabelColumn(x *xorm.Engine) error {
	type Label struct {
		OrgID int64 `xorm:"INDEX"`
	}

	if err := x.Sync(new(Label)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
