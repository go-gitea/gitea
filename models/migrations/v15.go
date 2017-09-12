// Copyright 2016 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

// UserV15 describes the added field for User
type UserV15 struct {
	KeepEmailPrivate        bool
	AllowCreateOrganization bool
}

// TableName will be invoked by XORM to customrize the table name
func (*UserV15) TableName() string {
	return "user"
}

func createAllowCreateOrganizationColumn(x *xorm.Engine) error {
	if err := x.Sync2(new(UserV15)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	} else if _, err = x.Where("type=0").Cols("allow_create_organization").Update(&UserV15{AllowCreateOrganization: true}); err != nil {
		return fmt.Errorf("set allow_create_organization: %v", err)
	}
	return nil
}
