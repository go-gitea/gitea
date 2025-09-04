// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"xorm.io/xorm"
)

func AddTriggerEventToActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		TriggerEvent string
	}

	return x.Sync(new(ActionRun))
}
