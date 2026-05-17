// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

func AddMatrixFieldsToActionRunJob(x *xorm.Engine) error {
	type ActionRunJob struct {
		RawMatrix    string         `xorm:"TEXT"`
		MatrixValues map[string]any `xorm:"JSON TEXT"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunJob))
	return err
}
