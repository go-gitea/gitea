// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base_test

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/names"
)

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}

func Test_DropTableColumnsWithForeignSchemaText(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip("only SQLite drops columns based on the stored schema of the table")
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		t.Skip("PrepareTestEnv did not yield a usable engine")
	}

	// identifiers quoted the standard SQL way instead of with backticks, as written by external tools that rebuild the database
	_, err := x.Exec(`CREATE TABLE "drop_test" ("id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, "repo_id" INTEGER NULL, "to_drop_column" TEXT NOT NULL)`)
	require.NoError(t, err)
	// a later xorm sync appends its own columns, leaving the stored schema with mixed quoting
	_, err = x.Exec("ALTER TABLE `drop_test` ADD COLUMN `keep_column` TEXT NULL")
	require.NoError(t, err)
	// a composite index over the dropped column: SQLite refuses to drop a column any index still covers
	_, err = x.Exec("CREATE INDEX `IDX_drop_test_repo_to_drop` ON `drop_test` (`repo_id`,`to_drop_column`)")
	require.NoError(t, err)
	_, err = x.Exec(`INSERT INTO "drop_test" ("repo_id", "keep_column", "to_drop_column") VALUES (1, 'keep', 'drop')`)
	require.NoError(t, err)

	sess := x.NewSession()
	defer sess.Close()
	require.NoError(t, sess.Begin())
	require.NoError(t, base.DropTableColumns(sess, "drop_test", "to_drop_column"))
	require.NoError(t, sess.Commit())

	exist, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "drop_test", "to_drop_column")
	require.NoError(t, err)
	require.False(t, exist, "to_drop_column must be gone")

	rows, err := x.Query(`SELECT "keep_column" FROM "drop_test"`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "keep", string(rows[0]["keep_column"]))
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
		if err := x.Sync(new(DropTest)); err != nil {
			t.Errorf("unable to create DropTest table: %v", err)
			return
		}

		sess := x.NewSession()
		if err := sess.Begin(); err != nil {
			sess.Close()
			t.Errorf("unable to begin transaction: %v", err)
			return
		}
		if err := base.DropTableColumns(sess, "drop_test", columns[i:]...); err != nil {
			sess.Close()
			t.Errorf("Unable to drop columns[%d:]: %s from drop_test: %v", i, columns[i:], err)
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
		for j := range columns[i+1:] {
			if err := x.Sync(new(DropTest)); err != nil {
				t.Errorf("unable to create DropTest table: %v", err)
				return
			}
			dropcols := append([]string{columns[i]}, columns[j+i+1:]...)
			sess := x.NewSession()
			if err := sess.Begin(); err != nil {
				sess.Close()
				t.Errorf("unable to begin transaction: %v", err)
				return
			}
			if err := base.DropTableColumns(sess, "drop_test", dropcols...); err != nil {
				sess.Close()
				t.Errorf("Unable to drop columns: %s from drop_test: %v", dropcols, err)
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
}
