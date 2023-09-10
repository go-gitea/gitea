// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint

import (
	"xorm.io/xorm"
)

func AddBlockOnOfficialReviewRequests(x *xorm.Engine) error {
	type ProtectedBranch struct {
		BlockOnOfficialReviewRequests bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
