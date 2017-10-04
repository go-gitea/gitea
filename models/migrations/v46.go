// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"

	"github.com/go-xorm/xorm"
)

func addIssueDependencyTables(x *xorm.Engine) (err error) {

	err = x.Sync(new(models.IssueDependency))

	if err != nil {
		return fmt.Errorf("Error creating issue_dependency_table column definition: %v", err)
	}

	return err
}
