// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddOwnerNameOnRepository(x *xorm.Engine) error {
	type Repository struct {
		OwnerName string
	}
	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE repository SET owner_name = (SELECT name FROM `user` WHERE `user`.id = repository.owner_id)")
	return err
}
