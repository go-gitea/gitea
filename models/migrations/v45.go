// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func removeIndexColumnFromRepoUnitTable(x *xorm.Engine) (err error) {
	if _, err := x.Exec("ALTER TABLE repo_unit DROP COLUMN index"); err != nil {
		return fmt.Errorf("DROP COLUMN index: %v", err)
	}
	return nil
}
