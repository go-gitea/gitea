// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import "code.gitea.io/gitea/models/db"

func AddBlockOnOfficialReviewRequests(x db.EngineMigration) error {
	type ProtectedBranch struct {
		BlockOnOfficialReviewRequests bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
