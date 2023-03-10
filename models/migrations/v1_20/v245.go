// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

// FixIncorrectProjectType: set individual project's type from 3(TypeOrganization) to 1(TypeIndividual)
func FixIncorrectProjectType(x *xorm.Engine) error {
	type User struct {
		ID   int64 `xorm:"pk autoincr"`
		Type int
	}

	const (
		UserTypeIndividual int = 0

		TypeIndividual   uint8 = 1
		TypeOrganization uint8 = 3
	)

	type Project struct {
		OwnerID int64 `xorm:"INDEX"`
		Type    uint8
		Owner   *User `xorm:"extends"`
	}

	ps := make([]Project, 0, 10)
	count, err := x.Table("project").
		//err := x.Table("project").
		Join("INNER", "user", "user.id = project.owner_id").
		Where("project.type = ? AND user.type = ?", TypeOrganization, UserTypeIndividual).
		//Find(&ps)
		Update(&Project{
			Type: TypeIndividual,
		})

	if err == nil {
		log.Debug("Updated %d projects to belong to a user instead of an organization", count)
		for _, p := range ps {
			fmt.Println(p)
			fmt.Println(p.Owner)
		}
	}

	return err
}
