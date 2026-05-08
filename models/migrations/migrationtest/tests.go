// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrationtest

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/testlogger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// PrepareTestEnv prepares the test environment and reset the database. The skip parameter should usually be 0.
// Provide models to be sync'd with the database - in particular any models you expect fixtures to be loaded from.
//
// fixtures in `models/migrations/fixtures/<TestName>` will be loaded automatically
func PrepareTestEnv(t *testing.T, skip int, syncModels ...any) (*xorm.Engine, func()) {
	t.Helper()
	ourSkip := 2
	ourSkip += skip
	deferFn := testlogger.PrintCurrentTest(t, ourSkip)
	giteaRoot := setting.GetGiteaTestSourceRoot()
	require.NoError(t, unittest.SyncDirs(filepath.Join(giteaRoot, "tests/gitea-repositories-meta"), setting.RepoRootPath))

	cleanup, err := unittest.ResetTestDatabase()
	if err != nil {
		t.Fatalf("unable to reset database: %v", err)
		return nil, deferFn
	}
	{
		oldDefer := deferFn
		deferFn = func() {
			cleanup()
			oldDefer()
		}
	}

	err = db.InitEngine(t.Context())
	if !assert.NoError(t, err) {
		return nil, deferFn
	}
	x := unittest.GetXORMEngine()
	{
		oldDefer := deferFn
		deferFn = func() {
			_ = x.Close()
			oldDefer()
		}
	}

	if len(syncModels) > 0 {
		if err := x.Sync(syncModels...); err != nil {
			t.Errorf("error during sync: %v", err)
			return x, deferFn
		}
	}

	fixturesDir := filepath.Join(giteaRoot, "models", "migrations", "fixtures", t.Name())

	if _, err := os.Stat(fixturesDir); err == nil {
		t.Logf("initializing fixtures from: %s", fixturesDir)
		if err := unittest.InitFixtures(
			unittest.FixturesOptions{
				Dir: fixturesDir,
			}); err != nil {
			t.Errorf("error whilst initializing fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
		if err := unittest.LoadFixtures(); err != nil {
			t.Errorf("error whilst loading fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error whilst checking for existence of fixtures: %v", err)
	} else {
		t.Logf("no fixtures found in: %s", fixturesDir)
	}

	return x, deferFn
}

func LoadTableSchemasMap(t *testing.T, x *xorm.Engine) map[string]*schemas.Table {
	tables, err := x.DBMetas()
	require.NoError(t, err)
	tableMap := make(map[string]*schemas.Table)
	for _, table := range tables {
		tableMap[table.Name] = table
	}
	return tableMap
}

func mainTest(m *testing.M) int {
	testlogger.Init()
	err := setting.PrepareIntegrationTestConfig()
	if err != nil {
		return testlogger.MainErrorf("Unable to prepare integration test config: %v", err)
	}
	setting.SetupGiteaTestEnv()

	if err = git.InitFull(); err != nil {
		return testlogger.MainErrorf("Unable to InitFull: %v", err)
	}
	setting.Database.SlowQueryThreshold = 0
	setting.LoadDBSetting()
	setting.InitLoggersForTest()
	return m.Run()
}

func MainTest(m *testing.M) {
	os.Exit(mainTest(m))
}
