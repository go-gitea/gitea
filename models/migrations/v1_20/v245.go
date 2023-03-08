// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

// FixIncorrectProjectType: set individual project's type from 3(TypeOrganization) to 1(TypeIndividual)
func FixIncorrectProjectType(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE project SET type = ? FROM user WHERE user.id = project.owner_id AND project.type = ? AND user.type = ?", 1, 3, 0)

	return err
}
