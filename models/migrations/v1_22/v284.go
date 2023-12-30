// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func UpdateExternalLoginUserProvider(x *xorm.Engine) error {
	type ExternalLoginUser struct {
		Provider string `xorm:"index VARCHAR(255)"`
	}

	return x.Sync(new(ExternalLoginUser))
}
