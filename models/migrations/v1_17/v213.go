// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func AddAllowMaintainerEdit(x *xorm.Engine) error {
	// PullRequest represents relation between pull request and repositories.
	type PullRequest struct {
		AllowMaintainerEdit bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PullRequest))
}
