// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddPublishedUnixToRelease(x *xorm.Engine) error {
	type Release struct {
		PublishedUnix int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	if err := x.Sync(new(Release)); err != nil {
		return err
	}
	// Initialize published_unix from created_unix for existing published releases
	_, err := x.Exec("UPDATE `release` SET published_unix = created_unix WHERE published_unix = 0 AND is_tag = ? AND is_draft = ?", false, false)
	return err
}
