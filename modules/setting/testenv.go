// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

var giteaTestSourceRoot *string

func GetGiteaTestSourceRoot() string {
	return *giteaTestSourceRoot
}

func SetupGiteaTestEnv() {
	if giteaTestSourceRoot != nil {
		return // already initialized
	}

	IsInTesting = true

	log.OsExiter = func(code int) {
		if code != 0 {
			// non-zero exit code (log.Fatal) shouldn't occur during testing, if it happens, show a full stacktrace for more details
			panic(fmt.Errorf("non-zero exit code during testing: %d", code))
		}
		os.Exit(0)
	}

	initGiteaRoot := func() string {
		giteaRoot := os.Getenv("GITEA_TEST_ROOT")
		if giteaRoot == "" {
			_, filename, _, _ := runtime.Caller(0)
			giteaRoot = filepath.Dir(filepath.Dir(filepath.Dir(filename)))
			fixturesDir := filepath.Join(giteaRoot, "models", "fixtures")
			if _, err := os.Stat(fixturesDir); err != nil {
				panic("in gitea source code directory, fixtures directory not found: " + fixturesDir)
			}
		}
		giteaTestSourceRoot = &giteaRoot
		return giteaRoot
	}

	initGiteaPaths := func() {
		appWorkPathBuiltin = *giteaTestSourceRoot
		AppWorkPath = appWorkPathBuiltin
		AppPath = filepath.Join(AppWorkPath, "gitea") + util.Iif(IsWindows, ".exe", "")
		StaticRootPath = AppWorkPath // need to load assets (options, public) from the source code directory for testing
	}

	initGiteaConf := func() string {
		// giteaConf (GITEA_CONF) must be relative because it is used in the git hooks as "$GITEA_ROOT/$GITEA_CONF"
		giteaConf := os.Getenv("GITEA_TEST_CONF")
		if giteaConf == "" {
			// if no GITEA_TEST_CONF, then it is in unit test, use a temp (non-existing / empty) config file
			giteaConf = "custom/conf/app-test-tmp.ini"
			customConfBuiltin = filepath.Join(AppWorkPath, giteaConf)
			CustomConf = customConfBuiltin
			_ = os.Remove(CustomConf)
		} else {
			// CustomConf must be absolute path to make tests pass,
			CustomConf = filepath.Join(AppWorkPath, giteaConf)
		}
		return giteaConf
	}

	cleanUpEnv := func() {
		// also unset unnecessary env vars for testing (only keep "GITEA_TEST_*" ones)
		UnsetUnnecessaryEnvVars()
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "GIT_") || (strings.HasPrefix(env, "GITEA_") && !strings.HasPrefix(env, "GITEA_TEST_")) {
				k, _, _ := strings.Cut(env, "=")
				_ = os.Unsetenv(k)
			}
		}
	}

	initWorkPathAndConfig := func() {
		// init paths and config system for testing
		getTestEnv := func(key string) string { return "" }
		InitWorkPathAndCommonConfig(getTestEnv, ArgWorkPathAndCustomConf{CustomConf: CustomConf})

		if err := PrepareAppDataPath(); err != nil {
			log.Fatal("Can not prepare APP_DATA_PATH: %v", err)
		}

		// register the dummy hash algorithm function used in the test fixtures
		_ = hash.Register("dummy", hash.NewDummyHasher)
		PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")
	}

	giteaRoot := initGiteaRoot()
	initGiteaPaths()
	giteaConf := initGiteaConf()
	cleanUpEnv()
	initWorkPathAndConfig()

	// TODO: some git repo hooks (test fixtures) still use these env variables, need to be refactored in the future
	_ = os.Setenv("GITEA_ROOT", giteaRoot)
	_ = os.Setenv("GITEA_CONF", giteaConf) // test fixture git hooks use "$GITEA_ROOT/$GITEA_CONF" in their scripts
}
