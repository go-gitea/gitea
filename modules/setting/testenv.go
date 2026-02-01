// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

func SetupGiteaTestEnv() string {
	giteaRoot := os.Getenv("GITEA_TEST_ROOT")
	if giteaRoot == "" {
		_, filename, _, _ := runtime.Caller(0)
		giteaRoot = filepath.Dir(filepath.Dir(filepath.Dir(filename)))
		fixturesDir := filepath.Join(giteaRoot, "models", "fixtures")
		if _, err := os.Stat(fixturesDir); err != nil {
			panic("in gitea source code directory, fixtures directory not found: " + fixturesDir)
		}
	}

	appWorkPathBuiltin = giteaRoot
	AppWorkPath = giteaRoot
	AppPath = filepath.Join(giteaRoot, "gitea") + util.Iif(IsWindows, ".exe", "")

	// giteaConf (GITEA_CONF) must be relative because it is used in the git hooks as "$GITEA_ROOT/$GITEA_CONF"
	giteaConf := os.Getenv("GITEA_TEST_CONF")
	if giteaConf == "" {
		// By default, use sqlite.ini for testing, then IDE like GoLand can start the test process with debugger.
		// It's easier for developers to debug bugs step by step with a debugger.
		// Notice: when doing "ssh push", Gitea executes sub processes, debugger won't work for the sub processes.
		giteaConf = "tests/sqlite.ini"
		_, _ = fmt.Fprintf(os.Stderr, "Environment variable GITEA_TEST_CONF not set - defaulting to %s\n", giteaConf)
		if !EnableSQLite3 {
			_, _ = fmt.Fprintf(os.Stderr, "sqlite3 requires: -tags sqlite,sqlite_unlock_notify\n")
			os.Exit(1)
		}
	}
	// CustomConf must be absolute path to make tests pass,
	CustomConf = filepath.Join(AppWorkPath, giteaConf)

	// also unset unnecessary env vars for testing (only keep "GITEA_TEST_*" ones)
	UnsetUnnecessaryEnvVars()
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GIT_") || (strings.HasPrefix(env, "GITEA_") && !strings.HasPrefix(env, "GITEA_TEST_")) {
			k, _, _ := strings.Cut(env, "=")
			_ = os.Unsetenv(k)
		}
	}

	// TODO: some git repo hooks (test fixtures) still use these env variables, need to be refactored in the future
	_ = os.Setenv("GITEA_ROOT", giteaRoot)
	_ = os.Setenv("GITEA_CONF", giteaConf) // test fixture git hooks use "$GITEA_ROOT/$GITEA_CONF" in their scripts

	return giteaRoot
}
