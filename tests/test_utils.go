// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tests

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers"

	"github.com/stretchr/testify/assert"
)

func InitTest() error {
	testlogger.Init()

	if os.Getenv("GITEA_TEST_CONF") == "" {
		// By default, use sqlite.ini for testing, then IDE like GoLand can start the test process with debugger.
		// It's easier for developers to debug bugs step by step with a debugger.
		// Notice: when doing "ssh push", Gitea executes sub processes, debugger won't work for the sub processes.
		giteaConf := "tests/sqlite.ini"
		_ = os.Setenv("GITEA_TEST_CONF", giteaConf)
		_, _ = fmt.Fprintf(os.Stderr, "Environment variable GITEA_TEST_CONF not set - defaulting to %s\n", giteaConf)
	}
	setting.SetupGiteaTestEnv()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"

	if err := git.InitFull(); err != nil {
		return err
	}

	setting.LoadDBSetting()
	if err := storage.Init(); err != nil {
		return err
	}

	switch {
	case setting.Database.Type.IsMySQL():
		{
			connType := util.Iif(strings.HasPrefix(setting.Database.Host, "/"), "unix", "tcp")
			db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@%s(%s)/",
				setting.Database.User, setting.Database.Passwd, connType, setting.Database.Host))
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + setting.Database.Name); err != nil {
				return err
			}
		}
	case setting.Database.Type.IsPostgreSQL():
		openPostgreSQL := func() (*sql.DB, error) {
			if strings.HasPrefix(setting.Database.Host, "/") {
				return sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
					setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
			}
			return sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		}

		// create database
		{
			db, err := openPostgreSQL()
			if err != nil {
				return err
			}
			defer db.Close()
			dbRows, err := db.Query(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", setting.Database.Name))
			if err != nil {
				return err
			}
			defer dbRows.Close()

			if !dbRows.Next() {
				if _, err = db.Exec("CREATE DATABASE " + setting.Database.Name); err != nil {
					return err
				}
			}
			// Check if we need to set up a specific schema
			if setting.Database.Schema == "" {
				break
			}
			db.Close()
		}

		// create schema
		{
			db, err := openPostgreSQL()
			if err != nil {
				return err
			}
			defer db.Close()

			schemaRows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
			if err != nil {
				return err
			}
			defer schemaRows.Close()

			if !schemaRows.Next() {
				// Create and set up a DB schema
				if _, err = db.Exec("CREATE SCHEMA " + setting.Database.Schema); err != nil {
					return err
				}
			}
		}

	case setting.Database.Type.IsMSSQL():
		{
			host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
			db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
				host, port, "master", setting.Database.User, setting.Database.Passwd))
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err = db.Exec(fmt.Sprintf("If(db_id(N'%s') IS NULL) BEGIN CREATE DATABASE %s; END;", setting.Database.Name, setting.Database.Name)); err != nil {
				return err
			}
		}
	}

	routers.InitWebInstalled(graceful.GetManager().HammerContext())
	return nil
}

func PrepareAttachmentsStorage(t testing.TB) {
	// prepare attachments directory and files
	assert.NoError(t, storage.Clean(storage.Attachments))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "attachments"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.Attachments, p, s, p)
		return err
	}))
}

func PrepareGitRepoDirectory(t testing.TB) {
	if !assert.NotEmpty(t, setting.RepoRootPath) {
		return
	}
	assert.NoError(t, unittest.SyncDirs(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/gitea-repositories-meta"), setting.RepoRootPath))
}

func PrepareArtifactsStorage(t testing.TB) {
	// prepare actions artifacts directory and files
	assert.NoError(t, storage.Clean(storage.ActionsArtifacts))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "artifacts"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.ActionsArtifacts, p, s, p)
		return err
	}))
}

func PrepareLFSStorage(t testing.TB) {
	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so init it with the storage API)
	lfsFixtures, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta"),
	})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects("", func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))
}

func PrepareCleanPackageData(t testing.TB) {
	// clear all package data
	assert.NoError(t, db.TruncateBeans(t.Context(),
		&packages_model.Package{},
		&packages_model.PackageVersion{},
		&packages_model.PackageFile{},
		&packages_model.PackageBlob{},
		&packages_model.PackageProperty{},
		&packages_model.PackageBlobUpload{},
		&packages_model.PackageCleanupRule{},
	))
	assert.NoError(t, storage.Clean(storage.Packages))
}

func PrepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	deferFn := PrintCurrentTest(t, util.OptionalArg(skip)+1)

	// load database fixtures
	assert.NoError(t, unittest.LoadFixtures())

	// do not add more Prepare* functions here, only call necessary ones in the related test functions
	PrepareGitRepoDirectory(t)
	PrepareLFSStorage(t)
	PrepareCleanPackageData(t)
	return deferFn
}

func PrintCurrentTest(t testing.TB, skip ...int) func() {
	t.Helper()
	return testlogger.PrintCurrentTest(t, util.OptionalArg(skip)+1)
}
