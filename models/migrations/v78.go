// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

func renameRepoIsBareToIsEmpty(x *xorm.Engine) error {
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		IsBare  bool
		IsEmpty bool `xorm:"INDEX"`
	}

	// First remove the index
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var err error
	if models.DbCfg.Type == core.POSTGRES || models.DbCfg.Type == core.SQLITE {
		_, err = sess.Exec("DROP INDEX IF EXISTS IDX_repository_is_bare")
	} else if models.DbCfg.Type == core.MSSQL {
		_, err = sess.Exec(`DECLARE @ConstraintName VARCHAR(256)
		DECLARE @SQL NVARCHAR(256)
		SELECT @ConstraintName = obj.name FROM sys.columns col LEFT OUTER JOIN sys.objects obj ON obj.object_id = col.default_object_id AND obj.type = 'D' WHERE col.object_id = OBJECT_ID('repository') AND obj.name IS NOT NULL AND col.name = 'is_bare'
		SET @SQL = N'ALTER TABLE [repository] DROP CONSTRAINT [' + @ConstraintName + N']'
		EXEC sp_executesql @SQL`)
		if err != nil {
			return err
		}
	} else if models.DbCfg.Type == core.MYSQL {
		indexes, err := sess.QueryString(`SHOW INDEX FROM repository WHERE KEY_NAME = 'IDX_repository_is_bare'`)
		if err != nil {
			return err
		}

		if len(indexes) >= 1 {
			_, err = sess.Exec("DROP INDEX IDX_repository_is_bare ON repository")
		}
	} else {
		_, err = sess.Exec("DROP INDEX IDX_repository_is_bare ON repository")
	}

	if err != nil {
		return fmt.Errorf("Drop index failed: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(Repository)); err != nil {
		return err
	}
	if _, err := sess.Exec("UPDATE repository SET is_empty = is_bare;"); err != nil {
		return err
	}

	if models.DbCfg.Type != core.SQLITE {
		_, err = sess.Exec("ALTER TABLE repository DROP COLUMN is_bare")
		if err != nil {
			return fmt.Errorf("Drop column failed: %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	if models.DbCfg.Type == core.SQLITE {
		log.Warn("TABLE repository's COLUMN is_bare should be DROP but sqlite is not supported, you could manually do that.")
	}
	return nil
}
