// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import "gitea.dev/modelmigration/base"

func AddAllowMaintainerEdit(x base.EngineMigration) error {
	// PullRequest represents relation between pull request and repositories.
	type PullRequest struct {
		AllowMaintainerEdit bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PullRequest))
}
