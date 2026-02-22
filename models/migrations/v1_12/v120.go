// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"xorm.io/xorm"
)

func AddOwnerNameOnRepository(x *xorm.Engine) error {
	type Repository struct {
		OwnerName string
	}
	if err := x.Sync(new(Repository)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE repository SET owner_name = (SELECT name FROM `user` WHERE `user`.id = repository.owner_id)")
	return err
}
