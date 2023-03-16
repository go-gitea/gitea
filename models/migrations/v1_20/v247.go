// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
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

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	sql, args, err := builder.ToSQL(
		builder.And(
			builder.Eq{"type": TypeOrganization},
			builder.And(builder.Eq{"owner_id": builder.Select("id").From("user").Where(builder.Eq{"type": UserTypeIndividual})}),
		),
	)
	if err != nil {
		return err
	}

	count, err := sess.Table("project").
		Where(sql, args...).
		Update(&Project{
			Type: TypeIndividual,
		})
	if err != nil {
		return err
	}
	log.Debug("Updated %d projects to belong to a user instead of an organization", count)

	return sess.Commit()
}
