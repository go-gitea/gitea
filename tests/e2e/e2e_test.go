// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This is primarily coped from /tests/integration/integration_test.go
//   TODO: Move common functions to shared file

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"

	_ "net/http/pprof" // Used for debugging if enabled and a web server is running
)

func TestMain(m *testing.M) {
	defer log.Close()

	initE2eTest()

	app := cli.NewApp()
	app.Action = cmd.CmdWeb.Action
	args := []string{"-c", setting.CustomConf}
	go app.Run(args)

	time.Sleep(10 * time.Second)

	os.Unsetenv("GIT_AUTHOR_NAME")
	os.Unsetenv("GIT_AUTHOR_EMAIL")
	os.Unsetenv("GIT_AUTHOR_DATE")
	os.Unsetenv("GIT_COMMITTER_NAME")
	os.Unsetenv("GIT_COMMITTER_EMAIL")
	os.Unsetenv("GIT_COMMITTER_DATE")

	err := unittest.InitFixtures(
		unittest.FixturesOptions{
			Dir: filepath.Join(filepath.Dir(setting.AppPath), "models/fixtures/"),
		},
	)
	if err != nil {
		fmt.Printf("Error initializing test database: %v\n", err)
		os.Exit(1)
	}

	exitVal := m.Run()

	writerCloser.Reset()

	if err = util.RemoveAll(setting.Indexer.IssuePath); err != nil {
		fmt.Printf("util.RemoveAll: %v\n", err)
		os.Exit(1)
	}
	if err = util.RemoveAll(setting.Indexer.RepoPath); err != nil {
		fmt.Printf("Unable to remove repo indexer: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitVal)
}

func initE2eTest() {
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}
	giteaBinary := "gitea"
	if runtime.GOOS == "windows" {
		giteaBinary += ".exe"
	}
	setting.AppPath = path.Join(giteaRoot, giteaBinary)
	if _, err := os.Stat(setting.AppPath); err != nil {
		fmt.Printf("Could not find gitea binary at %s\n", setting.AppPath)
		os.Exit(1)
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
	setting.LoadForTest()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"
	_ = util.RemoveAll(repo_module.LocalCopyPath())

	if err := git.InitOnceWithSync(context.Background()); err != nil {
		log.Fatal("git.InitOnceWithSync: %v", err)
	}
	git.CheckLFSVersion()

	setting.InitDBConfig()
	if err := storage.Init(); err != nil {
		fmt.Printf("Init storage failed: %v", err)
		os.Exit(1)
	}

	switch {
	case setting.Database.UseMySQL:
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
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

	// routers.GlobalInitInstalled(graceful.GetManager().HammerContext())
}

func prepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	ourSkip := 2
	if len(skip) > 0 {
		ourSkip += skip[0]
	}
	deferFn := PrintCurrentTest(t, ourSkip)
	assert.NoError(t, unittest.LoadFixtures())
	assert.NoError(t, util.RemoveAll(setting.RepoRootPath))
	assert.NoError(t, unittest.CopyDir(path.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))
	assert.NoError(t, git.InitOnceWithSync(context.Background())) // the gitconfig has been removed above, so sync the gitconfig again
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

	return deferFn
}

func TestE2e(t *testing.T) {
	defer prepareTestEnv(t)()
	cmd := exec.Command("npx", "playwright", "test")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GITEA_URL=%s", setting.AppURL))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error("%v", out.String())
		log.Fatal("Playwright Failed: %s", err)
	} else {
		log.Info("%v", out.String())
	}
}
