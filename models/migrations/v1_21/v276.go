// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	actions_model "code.gitea.io/gitea/models/actions"

	"xorm.io/xorm"
)

func AddPermissions(x *xorm.Engine) error {
	type ActionRunJob struct {
		Permissions actions_model.Permissions `xorm:"JSON TEXT"`
	}

	return x.Sync(new(ActionRunJob))
}
