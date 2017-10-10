// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
	"time"
)

func addIssueDependencyTables(x *xorm.Engine) (err error) {

	type issueDependency struct {
		ID           int64     `xorm:"pk autoincr"`
		UserID       int64     `xorm:"UNIQUE(watch) NOT NULL"`
		IssueID      int64     `xorm:"UNIQUE(watch) NOT NULL"`
		DependencyID int64     `xorm:"UNIQUE(watch) NOT NULL"`
		Created      time.Time `xorm:"-"`
		CreatedUnix  int64     `xorm:"INDEX created"`
		Updated      time.Time `xorm:"-"`
		UpdatedUnix  int64     `xorm:"updated"`
	}

	err = x.Sync(new(issueDependency))

	if err != nil {
		return fmt.Errorf("Error creating issue_dependency_table column definition: %v", err)
	}

	return err
}
