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

	if err := x.Sync(new(ActionRunner)); err != nil {
		return err
	}

	// update all records to set ephemeral to false
	_, err := x.Exec("UPDATE `action_runner` SET `ephemeral` = false WHERE `ephemeral` IS NULL")
	return err
}
