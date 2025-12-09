// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"xorm.io/xorm"
)

func AddIndexForAccessToken(x *xorm.Engine) error {
	type AccessToken struct {
		TokenLastEight string `xorm:"INDEX token_last_eight"`
	}

	return x.Sync(new(AccessToken))
}
