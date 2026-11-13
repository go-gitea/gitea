// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base_test

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/names"
)

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}

func Test_DropTableColumns(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	// FIXME: this logic seems wrong. Need to add an assertion here in the future, but it seems causing failure.
	if x == nil || t.Failed() {
		t.Skip("PrepareTestEnv did not yield a usable engine")
	}

	type DropTest struct {
		ID            int64 `xorm:"pk autoincr"`
		FirstColumn   string
		ToDropColumn  string `xorm:"unique"`
		AnotherColumn int64
		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	columns := []string{
		"first_column",
		"to_drop_column",
		"another_column",
		"created_unix",
		"updated_unix",
	}

	x.SetMapper(names.GonicMapper{})

	for i := range columns {
		for j := i; j < len(columns); j++ {
			if err := x.Sync(new(DropTest)); err != nil {
				t.Errorf("unable to create DropTest table: %v", err)
				return
			}
			dropcols := columns[i:]
			if j > i {
				dropcols = append([]string{columns[i]}, columns[j:]...)
			}
			sess := x.NewSession()
			if err := sess.Begin(); err != nil {
				sess.Close()
				t.Errorf("unable to begin transaction: %v", err)
				return
			}
			if err := base.DropTableColumns(sess, "drop_test", dropcols...); err != nil {
				sess.Close()
				if j == i {
					t.Errorf("Unable to drop columns[%d:]: %s from drop_test: %v", i, columns[i:], err)
				} else {
					t.Errorf("Unable to drop columns: %s from drop_test: %v", dropcols, err)
				}
				return
			}
			if err := sess.Commit(); err != nil {
				sess.Close()
				t.Errorf("unable to commit transaction: %v", err)
				return
			}
			sess.Close()
			if err := x.DropTables(new(DropTest)); err != nil {
				t.Errorf("unable to drop table: %v", err)
				return
			}
		}
	}

	if !setting.Database.Type.IsSQLite3() {
		return
	}
	for _, sql := range []string{
		"CREATE TABLE `drop_test` (`id` INTEGER PRIMARY KEY, \"keep_column\" TEXT, \"to_drop_column\" TEXT)",
		"CREATE INDEX `IDX_drop_test_composite` ON `drop_test` (`id`, `to_drop_column`)",
		"INSERT INTO `drop_test` (`id`, `keep_column`) VALUES (1, 'keep')",
	} {
		_, err := x.Exec(sql)
		require.NoError(t, err)
	}
	require.NoError(t, base.EngineTransaction(x, func(sess base.Session) error {
		return base.DropTableColumns(sess, "drop_test", "to_drop_column", "missing_column")
	}))
	exist, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "drop_test", "to_drop_column")
	require.NoError(t, err)
	assert.False(t, exist)
	rows, err := x.Query("SELECT `keep_column` FROM `drop_test`")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "keep", string(rows[0]["keep_column"]))
}
