// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import "xorm.io/xorm"

func AddArchivedUnixToRepository(x *xorm.Engine) error {
	type Repository struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"Default 0"`
	}

	return x.Sync(new(Repository))
}