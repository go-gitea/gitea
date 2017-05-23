// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

// ActionV34 describes the removed fields
type ActionV34 struct {
	ActUserName  string `xorm:"-"`
	RepoUserName string `xorm:"-"`
	RepoName     string `xorm:"-"`
}

// TableName will be invoked by XORM to customize the table name
func (*ActionV34) TableName() string {
	return "action"
}

func removeActionColumns(x *xorm.Engine) (err error) {
	if err = x.Sync(new(ActionV34)); err != nil {
		return fmt.Errorf("Sync: %v", err)
	}
	return nil
}
