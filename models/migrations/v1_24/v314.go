// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"xorm.io/xorm"
)

func AddEphemeralToActionRunner(x *xorm.Engine) error {
	type ActionRunner struct {
		Ephemeral bool `xorm:"ephemeral"`
	}

	return x.Sync(new(ActionRunner))
}
