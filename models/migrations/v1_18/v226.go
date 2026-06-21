// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"gitea.dev/models/db"

	"xorm.io/builder"
)

func FixPackageSemverField(x db.EngineMigration) error {
	_, err := x.Exec(builder.Update(builder.Eq{"semver_compatible": false}).From("`package`").Where(builder.In("`type`", "conan", "generic")))
	return err
}
