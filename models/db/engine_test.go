// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	_ "code.gitea.io/gitea/cmd" // for TestPrimaryKeys

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func TestDumpDatabase(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	dir := t.TempDir()

	type Version struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}
	assert.NoError(t, db.GetEngine(db.DefaultContext).Sync(new(Version)))

	for _, dbType := range setting.SupportedDatabaseTypes {
		assert.NoError(t, db.DumpDatabase(filepath.Join(dir, dbType+".sql"), dbType))
	}
}

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	countBefore, err := db.GetEngine(db.DefaultContext).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(db.DefaultContext).Insert(&issues_model.PullRequest{IssueID: 1000}, &issues_model.PullRequest{IssueID: 1001}, &issues_model.PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := db.CountOrphanedObjects(db.DefaultContext, "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = db.DeleteOrphanedObjects(db.DefaultContext, "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(db.DefaultContext).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)
	assert.EqualValues(t, countBefore, countAfter)
}

func TestPrimaryKeys(t *testing.T) {
	// Some dbs require that all tables have primary keys, see
	//   https://github.com/go-gitea/gitea/issues/21086
	//   https://github.com/go-gitea/gitea/issues/16802
	// To avoid creating tables without primary key again, this test will check them.
	// Import "code.gitea.io/gitea/cmd" to make sure each db.RegisterModel in init functions has been called.

	beans, err := db.NamesToBean()
	if err != nil {
		t.Fatal(err)
	}

	whitelist := map[string]string{
		"the_table_name_to_skip_checking": "Write a note here to explain why",
	}

	for _, bean := range beans {
		table, err := db.TableInfo(bean)
		if err != nil {
			t.Fatal(err)
		}
		if why, ok := whitelist[table.Name]; ok {
			t.Logf("ignore %q because %q", table.Name, why)
			continue
		}
		if len(table.PrimaryKeys) == 0 {
			t.Errorf("table %q has no primary key", table.Name)
		}
	}
}

func TestSlowQuery(t *testing.T) {
	lc, cleanup := test.NewLogChecker("slow-query")
	lc.StopMark("[Slow SQL Query]")
	defer cleanup()

	e := db.GetEngine(db.DefaultContext)
	engine, ok := e.(*xorm.Engine)
	assert.True(t, ok)

	// It's not possible to clean this up with XORM, but it's luckily not harmful
	// to leave around.
	engine.AddHook(&db.SlowQueryHook{
		Treshold: time.Second * 10,
		Logger:   log.GetLogger("slow-query"),
	})

	// NOOP query.
	e.Exec("SELECT 1 WHERE false;")

	_, stopped := lc.Check(100 * time.Millisecond)
	assert.False(t, stopped)

	engine.AddHook(&db.SlowQueryHook{
		Treshold: 0, // Every query should be logged.
		Logger:   log.GetLogger("slow-query"),
	})

	// NOOP query.
	e.Exec("SELECT 1 WHERE false;")

	_, stopped = lc.Check(100 * time.Millisecond)
	assert.True(t, stopped)
}
