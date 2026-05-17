// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

func AddCancellingSupportToActionRunner(x *xorm.Engine) error {
	type ActionRunner struct {
		HasCancellingSupport bool `xorm:"has_cancelling_support NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunner))
	return err
}
