// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "xorm.io/xorm"

func AddOriginalAuthorOnMigratedReleases(x *xorm.Engine) error {
	type Release struct {
		ID               int64
		OriginalAuthor   string
		OriginalAuthorID int64 `xorm:"index"`
	}

	return x.Sync(new(Release))
}
