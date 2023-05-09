// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers"

	"github.com/stretchr/testify/assert"
)

func exitf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}

func InitTest(requireGitea bool) {
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		exitf("Environment variable $GITEA_ROOT not set")
	}
	setting.AppWorkPath = giteaRoot
	if requireGitea {
		giteaBinary := "gitea"
		if setting.IsWindows {
			giteaBinary += ".exe"
		}
		setting.AppPath = path.Join(giteaRoot, giteaBinary)
		if _, err := os.Stat(setting.AppPath); err != nil {
			exitf("Could not find gitea binary at %s", setting.AppPath)
		}
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		// By default, use sqlite.ini for testing, then IDE like GoLand can start the test process with debugger.
		// It's easier for developers to debug bugs step by step with a debugger.
		// Notice: when doing "ssh push", Gitea executes sub processes, debugger won't work for the sub processes.
		giteaConf = "tests/sqlite.ini"
		_ = os.Setenv("GITEA_CONF", giteaConf)
		fmt.Printf("Environment variable $GITEA_CONF not set, use default: %s\n", giteaConf)
		if !setting.EnableSQLite3 {
			exitf(`sqlite3 requires: import _ "github.com/mattn/go-sqlite3" or -tags sqlite,sqlite_unlock_notify`)
		}
	}

	setting.IsInTesting = true

	if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.SetCustomPathAndConf("", "", "")
	unittest.InitSettings()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"
	_ = util.RemoveAll(repo_module.LocalCopyPath())

	if err := git.InitFull(context.Background()); err != nil {
		log.Fatal("git.InitOnceWithSync: %v", err)
	}

	setting.LoadDBSetting()
	if err := storage.Init(); err != nil {
		exitf("Init storage failed: %v", err)
	}

	switch {
	case setting.Database.Type.IsMySQL():
		connType := "tcp"
		if len(setting.Database.Host) > 0 && setting.Database.Host[0] == '/' { // looks like a unix socket
			connType = "unix"
		}

		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@%s(%s)/",
			setting.Database.User, setting.Database.Passwd, connType, setting.Database.Host))
		defer db.Close()
		if err != nil {
			log.Fatal("sql.Open: %v", err)
		}
		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", setting.Database.Name)); err != nil {
			log.Fatal("db.Exec: %v", err)
		}
	case setting.Database.Type.IsPostgreSQL():
		var db *sql.DB
		var err error
		if setting.Database.Host[0] == '/' {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
		} else {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		}

		defer db.Close()
		if err != nil {
			log.Fatal("sql.Open: %v", err)
		}
		dbrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", setting.Database.Name))
		if err != nil {
			log.Fatal("db.Query: %v", err)
		}
		defer dbrows.Close()

		if !dbrows.Next() {
			if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database.Name)); err != nil {
				log.Fatal("db.Exec: CREATE DATABASE: %v", err)
			}
		}
		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) == 0 {
			break
		}
		db.Close()

		if setting.Database.Host[0] == '/' {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
		} else {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		}
		// This is a different db object; requires a different Close()
		defer db.Close()
		if err != nil {
			log.Fatal("sql.Open: %v", err)
		}
		schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
		if err != nil {
			log.Fatal("db.Query: %v", err)
		}
		defer schrows.Close()

		if !schrows.Next() {
			// Create and setup a DB schema
			if _, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", setting.Database.Schema)); err != nil {
				log.Fatal("db.Exec: CREATE SCHEMA: %v", err)
			}
		}

	case setting.Database.Type.IsMSSQL():
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		if err != nil {
			log.Fatal("sql.Open: %v", err)
		}
		if _, err := db.Exec(fmt.Sprintf("If(db_id(N'%s') IS NULL) BEGIN CREATE DATABASE %s; END;", setting.Database.Name, setting.Database.Name)); err != nil {
			log.Fatal("db.Exec: %v", err)
		}
		defer db.Close()
	}

	routers.GlobalInitInstalled(graceful.GetManager().HammerContext())
}

func PrepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	ourSkip := 2
	if len(skip) > 0 {
		ourSkip += skip[0]
	}
	deferFn := PrintCurrentTest(t, ourSkip)

	// load database fixtures
	assert.NoError(t, unittest.LoadFixtures())

	// load git repo fixtures
	assert.NoError(t, util.RemoveAll(setting.RepoRootPath))
	assert.NoError(t, unittest.CopyDir(path.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))
	ownerDirs, err := os.ReadDir(setting.RepoRootPath)
	if err != nil {
		assert.NoError(t, err, "unable to read the new repo root: %v\n", err)
	}
	for _, ownerDir := range ownerDirs {
		if !ownerDir.Type().IsDir() {
			continue
		}
		repoDirs, err := os.ReadDir(filepath.Join(setting.RepoRootPath, ownerDir.Name()))
		if err != nil {
			assert.NoError(t, err, "unable to read the new repo root: %v\n", err)
		}
		for _, repoDir := range repoDirs {
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "pack"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "info"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "heads"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "tag"), 0o755)
		}
	}

	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so we init it with the storage API)
	lfsFixtures, err := storage.NewStorage("", storage.LocalStorageConfig{Path: path.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta")})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects("", func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))

	// clear all package data
	assert.NoError(t, db.TruncateBeans(db.DefaultContext,
		&packages_model.Package{},
		&packages_model.PackageVersion{},
		&packages_model.PackageFile{},
		&packages_model.PackageBlob{},
		&packages_model.PackageProperty{},
		&packages_model.PackageBlobUpload{},
		&packages_model.PackageCleanupRule{},
	))
	assert.NoError(t, storage.Clean(storage.Packages))

	return deferFn
}

func PrintCurrentTest(t testing.TB, skip ...int) func() {
	if len(skip) == 1 {
		skip = []int{skip[0] + 1}
	}
	return testlogger.PrintCurrentTest(t, skip...)
}

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...interface{}) {
	testlogger.Printf(format, args...)
}

func init() {
	log.Register("test", testlogger.NewTestLogger)
}
