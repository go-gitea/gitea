// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/tempdir"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// TestOptions represents test options
type TestOptions struct {
	FixtureFiles []string
	SetUp        func() error // SetUp will be executed before all tests in this package
	TearDown     func() error // TearDown will be executed after all tests in this package
}

// MainTest a reusable TestMain(..) function for unit tests that need to use a
// test database. Creates the test database, and sets necessary settings.
func MainTest(m *testing.M, testOptsArg ...*TestOptions) {
	os.Exit(mainTest(m, testOptsArg...))
}

func mainTest(m *testing.M, testOptsArg ...*TestOptions) int {
	testOpts := util.OptionalArg(testOptsArg, &TestOptions{})

	tempWorkPath, tempCleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("unit-test-dir-")
	if err != nil {
		return testlogger.MainErrorf("Failed to create temp dir for unit test: %v", err)
	}
	defer tempCleanup()

	defer setting.MockBuiltinPaths(tempWorkPath, "", "")()
	setting.SetupGiteaTestEnv()

	giteaRoot := setting.GetGiteaTestSourceRoot()
	fixturesOpts := FixturesOptions{Dir: filepath.Join(giteaRoot, "models", "fixtures"), Files: testOpts.FixtureFiles}
	if err := CreateTestEngine(fixturesOpts); err != nil {
		return testlogger.MainErrorf("Error creating test database engine: %v", err)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.Domain = "try.gitea.io"
	setting.RunUser = "runuser"
	setting.SSH.User = "sshuser"
	setting.SSH.BuiltinServerUser = "builtinuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.Database.Type = "sqlite3"
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"
	setting.GravatarSource = "https://secure.gravatar.com/avatar/"
	setting.IncomingEmail.ReplyToAddress = "incoming+%{token}@localhost"

	config.SetDynGetter(system.NewDatabaseDynKeyGetter())

	if err = cache.Init(); err != nil {
		return testlogger.MainErrorf("cache.Init: %v", err)
	}
	if err = storage.Init(); err != nil {
		return testlogger.MainErrorf("storage.Init: %v", err)
	}
	if err = SyncDirs(filepath.Join(giteaRoot, "tests", "gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		return testlogger.MainErrorf("util.SyncDirs: %v", err)
	}

	if err = git.InitFull(); err != nil {
		return testlogger.MainErrorf("git.Init: %v", err)
	}

	if testOpts.SetUp != nil {
		if err := testOpts.SetUp(); err != nil {
			return testlogger.MainErrorf("set up failed: %v", err)
		}
	}

	exitStatus := m.Run()

	if testOpts.TearDown != nil {
		if err := testOpts.TearDown(); err != nil {
			return testlogger.MainErrorf("tear down failed: %v", err)
		}
	}
	return exitStatus
}

// FixturesOptions fixtures needs to be loaded options
type FixturesOptions struct {
	Dir   string
	Files []string
}

// CreateTestEngine creates a memory database and loads the fixture data from fixturesDir
func CreateTestEngine(opts FixturesOptions) error {
	x, err := xorm.NewEngine("sqlite3", "file::memory:?cache=shared&_txlock=immediate")
	if err != nil {
		if strings.Contains(err.Error(), "unknown driver") {
			return fmt.Errorf("sqlite3 requires: -tags sqlite,sqlite_unlock_notify\n%w", err)
		}
		return err
	}
	x.SetMapper(names.GonicMapper{})
	db.SetDefaultEngine(context.Background(), x)

	if err = db.SyncAllTables(); err != nil {
		return err
	}
	switch os.Getenv("GITEA_TEST_LOG_SQL") {
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
	metaPath := filepath.Join(setting.GetGiteaTestSourceRoot(), "tests", "gitea-repositories-meta")
	assert.NoError(t, SyncDirs(metaPath, setting.RepoRootPath))
}
