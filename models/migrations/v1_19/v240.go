// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

func AddArchivedUnixForRepository(x *xorm.Engine) error {
	// all previous tokens have `all` and `sudo` scopes
	_, err := x.Exec("UPDATE repository SET archived_unix = 0 WHERE archived_unix IS NULL")
	return err
}
