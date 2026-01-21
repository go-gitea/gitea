// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddDefaultPRBaseBranchToRepository(x *xorm.Engine) error {
	type Repository struct {
		DefaultPRBaseBranch string
	}
	return x.Sync(new(Repository))
}
