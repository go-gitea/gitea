// Copyright 2016 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

type UserV15 struct {
	AllowCreateOrganization bool
}

func (*UserV15) TableName() string {
	return "user"
}

func createAllowCreateOrganizationColumn(x *xorm.Engine) error {
	return x.Sync2(new(UserV15))
}
