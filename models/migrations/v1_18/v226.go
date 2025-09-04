// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func FixPackageSemverField(x *xorm.Engine) error {
	_, err := x.Exec(builder.Update(builder.Eq{"semver_compatible": false}).From("`package`").Where(builder.In("`type`", "conan", "generic")))
	return err
}
