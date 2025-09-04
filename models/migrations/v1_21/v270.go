// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"xorm.io/xorm"
)

func FixPackagePropertyTypo(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec(`UPDATE package_property SET name = 'rpm.metadata' WHERE name = 'rpm.metdata'`); err != nil {
		return err
	}
	if _, err := sess.Exec(`UPDATE package_property SET name = 'conda.metadata' WHERE name = 'conda.metdata'`); err != nil {
		return err
	}

	return sess.Commit()
}
