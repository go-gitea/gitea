// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

func NoopDBMigrations(x *xorm.Engine) error {
	return nil
}
