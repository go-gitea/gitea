// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"path/filepath"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	_ "gitea.dev/cmd" // for TestPrimaryKeys

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestDumpDatabase(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	dir := t.TempDir()

	type Version struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}
	assert.NoError(t, db.GetEngine(t.Context()).Sync(new(Version)))

	for _, dbType := range setting.SupportedDatabaseTypes {
		assert.NoError(t, db.DumpDatabase(filepath.Join(dir, dbType+".sql"), setting.DatabaseType(dbType)))
	}
}

func TestSyncAllTablesKeepsSameColumnIndexes(t *testing.T) {
	// action declares c_u and c_u_d with the same columns in different order
	require.NoError(t, unittest.PrepareTestDatabase())
	x := db.GetXORMEngineForTesting()

	table, err := x.TableInfo(new(activities_model.Action))
	require.NoError(t, err)
	declared := map[string][]string{}
	for name, index := range table.Indexes {
		declared[name] = index.Cols
	}
	require.Contains(t, declared, "c_u")
	require.Contains(t, declared, "c_u_d")

	existing := func() (map[string][]string, map[string]*schemas.Index) {
		indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), table.Name)
		require.NoError(t, err)
		cols := map[string][]string{}
		for name, index := range indexes {
			cols[name] = index.Cols
		}
		return cols, indexes
	}

	for range 10 {
		require.NoError(t, db.SyncAllTables())
		cols, _ := existing()
		require.Equal(t, declared, cols)
	}

	for _, name := range []string{"c_u", "c_u_d"} {
		_, indexes := existing()
		_, err = x.Exec(x.Dialect().DropIndexSQL(table.Name, indexes[name]))
		require.NoError(t, err)
		require.NoError(t, db.SyncAllTables())
		cols, _ := existing()
		assert.Equal(t, declared, cols, "index %s should be restored", name)
	}
}

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	countBefore, err := db.GetEngine(t.Context()).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(t.Context()).Insert(&issues_model.PullRequest{IssueID: 1000}, &issues_model.PullRequest{IssueID: 1001}, &issues_model.PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := db.CountOrphanedObjects(t.Context(), "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = db.DeleteOrphanedObjects(t.Context(), "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(t.Context()).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)
	assert.Equal(t, countBefore, countAfter)
}

func TestPrimaryKeys(t *testing.T) {
	// Some dbs require that all tables have primary keys, see
	//   https://github.com/go-gitea/gitea/issues/21086
	//   https://github.com/go-gitea/gitea/issues/16802
	// To avoid creating tables without primary key again, this test will check them.
	// Import "gitea.dev/cmd" to make sure each db.RegisterModel in init functions has been called.

	beans, err := db.NamesToBean()
	require.NoError(t, err)

	whitelist := map[string]string{
		"the_table_name_to_skip_checking": "Write a note here to explain why",
	}

	for _, bean := range beans {
		table, err := db.GetXORMEngineForTesting().TableInfo(bean)
		if err != nil {
			t.Fatal(err)
		}
		if why, ok := whitelist[table.Name]; ok {
			t.Logf("ignore %q because %q", table.Name, why)
			continue
		}
		assert.NotEmpty(t, table.PrimaryKeys, "table %q has no primary key", table.Name)
	}
}
