// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import "xorm.io/xorm"

func AddIndexForReleaseSha1(x *xorm.Engine) error {
	type Release struct {
		Sha1 string `xorm:"INDEX VARCHAR(64)"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(Release))
	return err
}
