// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

// UpdateActionsRefIndex updates the index of actions ref field
func UpdateActionsRefIndex(x *xorm.Engine) error {
	type ActionRun struct {
		Ref string `xorm:"index"` // the commit/tag/… causing the run
	}
	return x.Sync(new(ActionRun))
}
