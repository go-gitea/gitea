// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func setCommentUpdatedWithCreated(x *xorm.Engine) (err error) {
	type Comment struct {
		UpdatedUnix int64
	}

	if err = x.Sync2(new(Comment)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	} else if _, err = x.Exec("UPDATE comment SET updated_unix = created_unix"); err != nil {
		return fmt.Errorf("set update_unix: %v", err)
	}
	return nil
}

// UserV14 describes the added fields for migrating from v13 -> v14
type UserV14 struct {
	DiffViewStyle string `xorm:"NOT NULL DEFAULT ''"`
}

// TableName will be invoked by XORM to customize the table name
func (*UserV14) TableName() string {
	return "user"
}

func createUserColumnDiffViewStyle(x *xorm.Engine) error {
	return x.Sync2(new(UserV14))
}
