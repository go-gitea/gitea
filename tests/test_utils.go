// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tests

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

func InitTest(requireGitea bool) {
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}
	if requireGitea {
		giteaBinary := "gitea"
		if runtime.GOOS == "windows" {
			giteaBinary += ".exe"
		}
		setting.AppPath = path.Join(giteaRoot, giteaBinary)
		if _, err := os.Stat(setting.AppPath); err != nil {
			fmt.Printf("Could not find gitea binary at %s\n", setting.AppPath)
			os.Exit(1)
		}
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		fmt.Println("Environment variable $GITEA_CONF not set")
		os.Exit(1)
	} else if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.SetCustomPathAndConf("", "", "")
	setting.InitProviderAndLoadCommonSettingsForTest()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"
	_ = util.RemoveAll(repo_module.LocalCopyPath())

	if err := git.InitFull(context.Background()); err != nil {
		log.Fatal("git.InitOnceWithSync: %v", err)
	}

	setting.LoadDBSetting()
	if err := storage.Init(); err != nil {
		fmt.Printf("Init storage failed: %v", err)
		os.Exit(1)
	}

	switch {
	case setting.Database.UseMySQL:
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
	case setting.Database.UsePostgreSQL:
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

	case setting.Database.UseMSSQL:
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

type RepoFixtureWatcher struct {
	changed *atomic.Bool
	watcher *atomic.Value
}

var repoFixtureWatcher = RepoFixtureWatcher{
	changed: &atomic.Bool{},
	watcher: &atomic.Value{},
}

func (r *RepoFixtureWatcher) PrepareRepoFixtures() error {
	_, err := os.Stat(setting.RepoRootPath)
	if err == nil && !r.changed.Load() {
		return nil
	}
	watcherVal := r.watcher.Load()
	if watcherVal != nil {
		if watcher, ok := watcherVal.(*fsnotify.Watcher); ok {
			watcher.Close()
		}
	}
	if err := resetFixtureRepositories(); err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		if err := filepath.WalkDir(setting.RepoRootPath, func(path string, _ fs.DirEntry, err error) error {
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			_ = watcher.Add(path)
			return nil
		}); err != nil {
			r.changed.Store(true)
			return
		}

		select {
		case _, ok := <-watcher.Events:
			if !ok {
				_ = watcher.Close()
				return
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				_ = watcher.Close()
				return
			}
		}
		_ = watcher.Close()
		r.changed.Store(true)
	}()

	return nil
}

func resetFixtureRepositories() error {
	if err := util.RemoveAll(setting.RepoRootPath); err != nil {
		return err
	}
	if err := unittest.CopyDir(path.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath); err != nil {
		return err
	}
	ownerDirs, err := os.ReadDir(setting.RepoRootPath)
	if err != nil {
		return fmt.Errorf("unable to read the new repo root: %w", err)
	}
	for _, ownerDir := range ownerDirs {
		if !ownerDir.Type().IsDir() {
			continue
		}
		repoDirs, err := os.ReadDir(filepath.Join(setting.RepoRootPath, ownerDir.Name()))
		if err != nil {
			return fmt.Errorf("unable to read the new repo root: %w", err)
		}
		for _, repoDir := range repoDirs {
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "pack"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "objects", "info"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "heads"), 0o755)
			_ = os.MkdirAll(filepath.Join(setting.RepoRootPath, ownerDir.Name(), repoDir.Name(), "refs", "tag"), 0o755)
		}
	}
	return nil
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
	assert.NoError(t, repoFixtureWatcher.PrepareRepoFixtures())

	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so we init it with the storage API)
	lfsFixtures, err := storage.NewStorage("", storage.LocalStorageConfig{Path: path.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta")})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects(func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))

	return deferFn
}

// resetFixtures flushes queues, reloads fixtures and resets test repositories within a single test.
// Most tests should call defer tests.PrepareTestEnv(t)() (or have onGiteaRun do that for them) but sometimes
// within a single test this is required
func ResetFixtures(t *testing.T) {
	assert.NoError(t, queue.GetManager().FlushAll(context.Background(), -1))

	// load database fixtures
	assert.NoError(t, unittest.LoadFixtures())

	// load git repo fixtures
	assert.NoError(t, repoFixtureWatcher.PrepareRepoFixtures())

	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so we init it with the storage API)
	lfsFixtures, err := storage.NewStorage("", storage.LocalStorageConfig{Path: path.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta")})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects(func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))
}
