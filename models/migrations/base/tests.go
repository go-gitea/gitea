// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// FIXME: this file shouldn't be in a normal package, it should only be compiled for tests

func removeAllWithRetry(dir string) error {
	var err error
	for range 20 {
		err = os.RemoveAll(dir)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func newXORMEngine(t *testing.T) (*xorm.Engine, error) {
	if err := db.InitEngine(t.Context()); err != nil {
		return nil, err
	}
	x := unittest.GetXORMEngine()
	return x, nil
}

func deleteDB() error {
	switch {
	case setting.Database.Type.IsSQLite3():
		if err := util.Remove(setting.Database.Path); err != nil {
			return err
		}
		return os.MkdirAll(path.Dir(setting.Database.Path), os.ModePerm)

	case setting.Database.Type.IsMySQL():
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec("DROP DATABASE IF EXISTS " + setting.Database.Name); err != nil {
			return err
		}

		if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + setting.Database.Name); err != nil {
			return err
		}
		return nil
	case setting.Database.Type.IsPostgreSQL():
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec("DROP DATABASE IF EXISTS " + setting.Database.Name); err != nil {
			return err
		}

		if _, err = db.Exec("CREATE DATABASE " + setting.Database.Name); err != nil {
			return err
		}
		db.Close()

		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) != 0 {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
			if err != nil {
				return err
			}
			defer db.Close()

			schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
			if err != nil {
				return err
			}
			defer schrows.Close()

			if !schrows.Next() {
				// Create and setup a DB schema
				_, err = db.Exec("CREATE SCHEMA " + setting.Database.Schema)
				if err != nil {
					return err
				}
			}

			// Make the user's default search path the created schema; this will affect new connections
			_, err = db.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema))
			if err != nil {
				return err
			}
			return nil
		}
	case setting.Database.Type.IsMSSQL():
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		if err != nil {
			return err
		}
		defer db.Close()

		if _, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS [%s]", setting.Database.Name)); err != nil {
			return err
		}
		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE [%s]", setting.Database.Name)); err != nil {
			return err
		}
	}

	return nil
}

// PrepareTestEnv prepares the test environment and reset the database. The skip parameter should usually be 0.
// Provide models to be sync'd with the database - in particular any models you expect fixtures to be loaded from.
//
// fixtures in `models/migrations/fixtures/<TestName>` will be loaded automatically
func PrepareTestEnv(t *testing.T, skip int, syncModels ...any) (*xorm.Engine, func()) {
	t.Helper()
	ourSkip := 2
	ourSkip += skip
	deferFn := testlogger.PrintCurrentTest(t, ourSkip)
	require.NoError(t, unittest.SyncDirs(filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))

	if err := deleteDB(); err != nil {
		t.Fatalf("unable to reset database: %v", err)
		return nil, deferFn
	}

	x, err := newXORMEngine(t)
	require.NoError(t, err)
	if x != nil {
		oldDefer := deferFn
		deferFn = func() {
			oldDefer()
			if err := x.Close(); err != nil {
				t.Errorf("error during close: %v", err)
			}
			if err := deleteDB(); err != nil {
				t.Errorf("unable to reset database: %v", err)
			}
		}
	}
	if err != nil {
		return x, deferFn
	}

	if len(syncModels) > 0 {
		if err := x.Sync(syncModels...); err != nil {
			t.Errorf("error during sync: %v", err)
			return x, deferFn
		}
	}

	fixturesDir := filepath.Join(filepath.Dir(setting.AppPath), "models", "migrations", "fixtures", t.Name())

	if _, err := os.Stat(fixturesDir); err == nil {
		t.Logf("initializing fixtures from: %s", fixturesDir)
		if err := unittest.InitFixtures(
			unittest.FixturesOptions{
				Dir: fixturesDir,
			}, x); err != nil {
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

func MainTest(m *testing.M) {
	testlogger.Init()
	setting.SetupGiteaTestEnv()

	tmpDataPath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("data")
	if err != nil {
		testlogger.Fatalf("Unable to create temporary data path %v\n", err)
	}
	defer cleanup()

	setting.AppDataPath = tmpDataPath

	unittest.InitSettingsForTesting()
	if err = git.InitFull(); err != nil {
		testlogger.Fatalf("Unable to InitFull: %v\n", err)
	}
	setting.LoadDBSetting()
	setting.InitLoggersForTest()

	exitStatus := m.Run()

	if err := removeAllWithRetry(setting.RepoRootPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "os.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}
