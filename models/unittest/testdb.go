// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittest

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

// giteaRoot a path to the gitea root
var (
	giteaRoot   string
	fixturesDir string
)

// FixturesDir returns the fixture directory
func FixturesDir() string {
	return fixturesDir
}

func fatalTestError(fmtStr string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

// MainTest a reusable TestMain(..) function for unit tests that need to use a
// test database. Creates the test database, and sets necessary settings.
func MainTest(m *testing.M, pathToGiteaRoot string, fixtureFiles ...string) {
	var err error

	giteaRoot = pathToGiteaRoot
	fixturesDir = filepath.Join(pathToGiteaRoot, "models", "fixtures")

	var opts FixturesOptions
	if len(fixtureFiles) == 0 {
		opts.Dir = fixturesDir
	} else {
		for _, f := range fixtureFiles {
			if len(f) != 0 {
				opts.Files = append(opts.Files, filepath.Join(fixturesDir, f))
			}
		}
	}

	if err = InitTestEngine(opts); err != nil {
		fatalTestError("Error creating test engine: %v\n", err)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.Database.UseSQLite3 = true
	setting.RepoRootPath, err = os.MkdirTemp(os.TempDir(), "repos")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppDataPath, err = os.MkdirTemp(os.TempDir(), "appdata")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppWorkPath = pathToGiteaRoot
	setting.StaticRootPath = pathToGiteaRoot
	setting.GravatarSourceURL, err = url.Parse("https://secure.gravatar.com/avatar/")
	if err != nil {
		fatalTestError("url.Parse: %v\n", err)
	}
	setting.Attachment.Storage.Path = filepath.Join(setting.AppDataPath, "attachments")

	setting.LFS.Storage.Path = filepath.Join(setting.AppDataPath, "lfs")

	setting.Avatar.Storage.Path = filepath.Join(setting.AppDataPath, "avatars")

	setting.RepoAvatar.Storage.Path = filepath.Join(setting.AppDataPath, "repo-avatars")

	setting.RepoArchive.Storage.Path = filepath.Join(setting.AppDataPath, "repo-archive")

	if err = storage.Init(); err != nil {
		fatalTestError("storage.Init: %v\n", err)
	}

	if err = util.RemoveAll(setting.RepoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = util.CopyDir(filepath.Join(pathToGiteaRoot, "integrations", "gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		fatalTestError("util.CopyDir: %v\n", err)
	}

	exitStatus := m.Run()
	if err = util.RemoveAll(setting.RepoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = util.RemoveAll(setting.AppDataPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}

func createDB(driverName, connStr string) error {
	driver := dialects.QueryDriver(driverName)
	if driver == nil {
		return fmt.Errorf("Unsupported driver name: %v", driver)
	}
	uri, err := driver.Parse(driverName, connStr)
	if err != nil {
		return err
	}
	dbType := uri.DBType
	dbName := uri.DBName
	dbSchema := uri.Schema

	switch dbType {
	case schemas.SQLITE: // ignore the creation
	case schemas.MSSQL:
		db, err := sql.Open(driverName, strings.Replace(connStr, dbName, "master", -1))
		if err != nil {
			return err
		}
		if _, err = db.Exec(fmt.Sprintf("If(db_id(N'%s') IS NULL) BEGIN CREATE DATABASE %s; END;", dbName, dbName)); err != nil {
			return fmt.Errorf("db.Exec: %v", err)
		}
		db.Close()
	case schemas.POSTGRES:
		db, err := sql.Open(driverName, connStr)
		if err != nil {
			return err
		}
		rows, err := db.Query(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", dbName))
		if err != nil {
			return fmt.Errorf("db.Query: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
				return fmt.Errorf("CREATE DATABASE: %v", err)
			}
		}
		if dbSchema != "" {
			if _, err = db.Exec("CREATE SCHEMA IF NOT EXISTS " + dbSchema); err != nil {
				return fmt.Errorf("CREATE SCHEMA: %v", err)
			}
		}
		db.Close()
	case schemas.MYSQL:
		db, err := sql.Open(driverName, strings.Replace(connStr, dbName, "mysql", -1))
		if err != nil {
			return err
		}
		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)); err != nil {
			return fmt.Errorf("db.Exec: %v", err)
		}
		db.Close()
	default:
		return errors.New("Unsupported database for unit test")
	}
	return nil
}

// FixturesOptions fixtures needs to be loaded options
type FixturesOptions struct {
	Dir   string
	Files []string
}

// InitTestEngine creates a memory database and loads the fixture data from fixturesDir
func InitTestEngine(opts FixturesOptions) error {
	var dbType = "sqlite3"
	var dbConnStr = "file::memory:?cache=shared&_txlock=immediate"

	if os.Getenv("GITEA_UNIT_TESTS_DB_TYPE") != "" {
		dbType = os.Getenv("GITEA_UNIT_TESTS_DB_TYPE")
	}

	if os.Getenv("GITEA_UNIT_TESTS_DB_CONNSTR") != "" {
		dbConnStr = os.Getenv("GITEA_UNIT_TESTS_DB_CONNSTR")
	}

	if err := createDB(dbType, dbConnStr); err != nil {
		return fmt.Errorf("createDB failed: %v", err)
	}

	var err error
	x, err := xorm.NewEngine(dbType, dbConnStr)
	if err != nil {
		return err
	}
	x.SetMapper(names.GonicMapper{})
	db.SetEngine(x)

	if err = db.SyncAllTables(); err != nil {
		return err
	}
	switch os.Getenv("GITEA_UNIT_TESTS_VERBOSE") {
	case "true", "1":
		x.ShowSQL(true)
	}

	return InitFixtures(opts)
}

// PrepareTestDatabase load test fixtures into test database
func PrepareTestDatabase() error {
	return LoadFixtures()
}

// PrepareTestEnv prepares the environment for unit tests. Can only be called
// by tests that use the above MainTest(..) function.
func PrepareTestEnv(t testing.TB) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, util.RemoveAll(setting.RepoRootPath))
	metaPath := filepath.Join(giteaRoot, "integrations", "gitea-repositories-meta")
	assert.NoError(t, util.CopyDir(metaPath, setting.RepoRootPath))
	base.SetupGiteaRoot() // Makes sure GITEA_ROOT is set
}
