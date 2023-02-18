// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

// ChangeProjectType: set ProjectType from 3(TypeOrganization/TypeIndividual) to 1(TypeUser)
func ChangeProjectType(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE project SET type = ? WHERE type = ?", 1, 3)
	return err
}
