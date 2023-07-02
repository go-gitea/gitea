// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddPinOrderToIssue(x *xorm.Engine) error {
	type Issue struct {
		PinOrder int `xorm:"DEFAULT 0"`
	}

	return x.Sync(new(Issue))
}
