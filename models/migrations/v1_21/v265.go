// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint
import (
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"xorm.io/xorm"
)

func RebuildRpmPackage(x *xorm.Engine) error {
	//x.Select("")
	distribution := rpm_module.RepositoryDefaultDistribution
	println(distribution)
	return nil
}
