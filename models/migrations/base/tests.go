// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package base

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

// PrepareTestEnv prepares the test environment and reset the database. The skip parameter should usually be 0.
// Provide models to be sync'd with the database - in particular any models you expect fixtures to be loaded from.
//
// fixtures in `models/migrations/fixtures/<TestName>` will be loaded automatically
func PrepareTestEnv(t *testing.T, skip int, syncModels ...interface{}) (*xorm.Engine, func()) {
	t.Helper()
	ourSkip := 2
	ourSkip += skip
	deferFn := PrintCurrentTest(t, ourSkip)
	assert.NoError(t, os.RemoveAll(setting.RepoRootPath))
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

	if err := deleteDB(); err != nil {
		t.Errorf("unable to reset database: %v", err)
		return nil, deferFn
	}

	x, err := newXORMEngine()
	assert.NoError(t, err)
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
		if err := x.Sync2(syncModels...); err != nil {
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
		if err := unittest.LoadFixtures(x); err != nil {
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

func MainTest(m *testing.M) {
	log.Register("test", NewTestLogger)
	_, filename, _, _ := runtime.Caller(0)
	prefix = strings.TrimSuffix(filename, "tests/testlogger.go")

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
		giteaConf = path.Join(filepath.Dir(setting.AppPath), "tests/sqlite.ini")
		fmt.Printf("Environment variable $GITEA_CONF not set - defaulting to %s\n", giteaConf)
	}

	if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	tmpDataPath, err := os.MkdirTemp("", "data")
	if err != nil {
		fmt.Printf("Unable to create temporary data path %v\n", err)
		os.Exit(1)
	}

	setting.AppDataPath = tmpDataPath

	setting.SetCustomPathAndConf("", "", "")
	setting.InitProviderAndLoadCommonSettingsForTest()
	if err = git.InitFull(context.Background()); err != nil {
		fmt.Printf("Unable to InitFull: %v\n", err)
		os.Exit(1)
	}
	setting.LoadDBSetting()
	setting.InitLogs(true)

	exitStatus := m.Run()

	if err := removeAllWithRetry(setting.RepoRootPath); err != nil {
		fmt.Fprintf(os.Stderr, "os.RemoveAll: %v\n", err)
	}
	if err := removeAllWithRetry(tmpDataPath); err != nil {
		fmt.Fprintf(os.Stderr, "os.RemoveAll: %v\n", err)
	}
	os.Exit(exitStatus)
}
