// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
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

func fatalTestError(fmtStr string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

// InitSettings initializes config provider and load common settings for tests
func InitSettings(extraConfigs ...string) {
	if setting.CustomConf == "" {
		setting.CustomConf = filepath.Join(setting.CustomPath, "conf/app-unittest-tmp.ini")
		_ = os.Remove(setting.CustomConf)
	}
	setting.InitCfgProvider(setting.CustomConf, strings.Join(extraConfigs, "\n"))
	setting.LoadCommonSettings()

	if err := setting.PrepareAppDataPath(); err != nil {
		log.Fatalf("Can not prepare APP_DATA_PATH: %v", err)
	}
	// register the dummy hash algorithm function used in the test fixtures
	_ = hash.Register("dummy", hash.NewDummyHasher)

	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")
}

// TestOptions represents test options
type TestOptions struct {
	GiteaRootPath string
	FixtureFiles  []string
	SetUp         func() error // SetUp will be executed before all tests in this package
	TearDown      func() error // TearDown will be executed after all tests in this package
}

// MainTest a reusable TestMain(..) function for unit tests that need to use a
// test database. Creates the test database, and sets necessary settings.
func MainTest(m *testing.M, testOpts *TestOptions) {
	setting.CustomPath = filepath.Join(testOpts.GiteaRootPath, "custom")
	InitSettings()

	var err error

	giteaRoot = testOpts.GiteaRootPath
	fixturesDir = filepath.Join(testOpts.GiteaRootPath, "models", "fixtures")

	var opts FixturesOptions
	if len(testOpts.FixtureFiles) == 0 {
		opts.Dir = fixturesDir
	} else {
		for _, f := range testOpts.FixtureFiles {
			if len(f) != 0 {
				opts.Files = append(opts.Files, filepath.Join(fixturesDir, f))
			}
		}
	}

	if err = CreateTestEngine(opts); err != nil {
		fatalTestError("Error creating test engine: %v\n", err)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.User = "sshuser"
	setting.SSH.BuiltinServerUser = "builtinuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.Database.Type = "sqlite3"
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"
	repoRootPath, err := os.MkdirTemp(os.TempDir(), "repos")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.RepoRootPath = repoRootPath
	appDataPath, err := os.MkdirTemp(os.TempDir(), "appdata")
	if err != nil {
		fatalTestError("TempDir: %v\n", err)
	}
	setting.AppDataPath = appDataPath
	setting.AppWorkPath = testOpts.GiteaRootPath
	setting.StaticRootPath = testOpts.GiteaRootPath
	setting.GravatarSource = "https://secure.gravatar.com/avatar/"

	setting.Attachment.Storage.Path = filepath.Join(setting.AppDataPath, "attachments")

	setting.LFS.Storage.Path = filepath.Join(setting.AppDataPath, "lfs")

	setting.Avatar.Storage.Path = filepath.Join(setting.AppDataPath, "avatars")

	setting.RepoAvatar.Storage.Path = filepath.Join(setting.AppDataPath, "repo-avatars")

	setting.RepoArchive.Storage.Path = filepath.Join(setting.AppDataPath, "repo-archive")

	setting.Packages.Storage.Path = filepath.Join(setting.AppDataPath, "packages")

	setting.Actions.LogStorage.Path = filepath.Join(setting.AppDataPath, "actions_log")

	setting.Git.HomePath = filepath.Join(setting.AppDataPath, "home")

	setting.IncomingEmail.ReplyToAddress = "incoming+%{token}@localhost"

	if err = storage.Init(); err != nil {
		fatalTestError("storage.Init: %v\n", err)
	}
	if err = system_model.Init(db.DefaultContext); err != nil {
		fatalTestError("models.Init: %v\n", err)
	}

	if err = util.RemoveAll(repoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = CopyDir(filepath.Join(testOpts.GiteaRootPath, "tests", "gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		fatalTestError("util.CopyDir: %v\n", err)
	}

	if err = git.InitFull(context.Background()); err != nil {
		fatalTestError("git.Init: %v\n", err)
	}
	ownerDirs, err := os.ReadDir(setting.RepoRootPath)
	if err != nil {
		fatalTestError("unable to read the new repo root: %v\n", err)
	}
	for _, ownerDir := range ownerDirs {
		if !ownerDir.Type().IsDir() {
			continue
		}
		repoDirs, err := os.ReadDir(filepath.Join(setting.RepoRootPath, ownerDir.Name()))
		if err != nil {
			fatalTestError("unable to read the new repo root: %v\n", err)
		}
		for _, repoDir := range repoDirs {
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "pack"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "info"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "heads"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "tag"), 0o755)
		}
	}

	if testOpts.SetUp != nil {
		if err := testOpts.SetUp(); err != nil {
			fatalTestError("set up failed: %v\n", err)
		}
	}

	exitStatus := m.Run()

	if testOpts.TearDown != nil {
		if err := testOpts.TearDown(); err != nil {
			fatalTestError("tear down failed: %v\n", err)
		}
	}

	if err = util.RemoveAll(repoRootPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	if err = util.RemoveAll(appDataPath); err != nil {
		fatalTestError("util.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
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
			return fmt.Errorf(`sqlite3 requires: import _ "github.com/mattn/go-sqlite3" or -tags sqlite,sqlite_unlock_notify%s%w`, "\n", err)
		}
		return err
	}
	x.SetMapper(names.GonicMapper{})
	db.SetDefaultEngine(context.Background(), x)

	if err = db.SyncAllTables(); err != nil {
		return err
	}
	switch os.Getenv("GITEA_UNIT_TESTS_LOG_SQL") {
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
	metaPath := filepath.Join(giteaRoot, "tests", "gitea-repositories-meta")
	assert.NoError(t, CopyDir(metaPath, setting.RepoRootPath))
	ownerDirs, err := os.ReadDir(setting.RepoRootPath)
	assert.NoError(t, err)
	for _, ownerDir := range ownerDirs {
		if !ownerDir.Type().IsDir() {
			continue
		}
		repoDirs, err := os.ReadDir(filepath.Join(setting.RepoRootPath, ownerDir.Name()))
		assert.NoError(t, err)
		for _, repoDir := range repoDirs {
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "pack"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "info"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "heads"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "tag"), 0o755)
		}
	}

	base.SetupGiteaRoot() // Makes sure GITEA_ROOT is set
}
