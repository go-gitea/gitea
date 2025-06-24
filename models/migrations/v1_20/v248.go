// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint:revive // underscore in migration packages isn't a large issue

import "xorm.io/xorm"

func AddVersionToActionRunner(x *xorm.Engine) error {
	type ActionRunner struct {
		Version string `xorm:"VARCHAR(64)"` // the version of act_runner
	}

	return x.Sync(new(ActionRunner))
}
