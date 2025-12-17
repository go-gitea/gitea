// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"xorm.io/xorm"
)

func AddVersionToActionRunTable(x *xorm.Engine) error {
	type ActionRun struct {
		Version int `xorm:"version default 0"`
	}
	return x.Sync(new(ActionRun))
}
