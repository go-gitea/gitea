// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"xorm.io/xorm"
)

func AddExclusiveOrderColumnToLabelTable(x *xorm.Engine) error {
	type Label struct {
		ExclusiveOrder int64 `xorm:"DEFAULT 0"`
	}

	return x.Sync(new(Label))
}
