// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateAuthorizationTokenTable(x *xorm.Engine) error {
	type AuthorizationToken struct {
		ID              int64  `xorm:"pk autoincr"`
		UID             int64  `xorm:"INDEX"`
		LookupKey       string `xorm:"INDEX UNIQUE"`
		HashedValidator string
		Expiry          timeutil.TimeStamp
	}

	return x.Sync(new(AuthorizationToken))
}
