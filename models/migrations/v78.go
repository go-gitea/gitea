// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"

	"github.com/go-xorm/xorm"
)

func renameRepoIsBareToIsEmpty(x *xorm.Engine) error {
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		IsBare  bool
		IsEmpty bool `xorm:"INDEX"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	var err error
	if models.DbCfg.Type == "mssql" {
		_, err = sess.Query("EXEC sp_rename 'repository.is_bare', 'is_empty', 'COLUMN'")
	} else {
		_, err = sess.Query("ALTER TABLE \"repository\" RENAME COLUMN \"is_bare\" TO \"is_empty\";")
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such column") {
			return nil
		}
		return fmt.Errorf("select repositories: %v", err)
	}

	return sess.Commit()
}
