// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"
)

func AddConcurrencyGroupToActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		ConcurrencyGroup string
	}

	return x.Sync(new(ActionRun))
}
