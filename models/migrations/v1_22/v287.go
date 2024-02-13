// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

type HookTask struct {
	PayloadVersion int
}

func AddPayloadVersionToHookTaskTable(x *xorm.Engine) error {
	// create missing column
	if err := x.Sync(new(HookTask)); err != nil {
		return err
	}
	// set payload_version to 1
	_, err := x.Cols("payload_version").Update(HookTask{PayloadVersion: 1})
	return err
}
