// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddRepoArchiveDownloadCount(x *xorm.Engine) error {
	type RepoArchiveDownloadCount struct {
		ID        int64 `xorm:"pk autoincr"`
		RepoID    int64 `xorm:"index unique(s)"`
		ReleaseID int64 `xorm:"index unique(s)"`
		Type      int   `xorm:"unique(s)"`
		Count     int64
	}

	return x.Sync(&RepoArchiveDownloadCount{})
}
