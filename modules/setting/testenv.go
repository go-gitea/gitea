// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/kballard/go-shellquote"
)

var giteaTestSourceRoot *string // intentionally use a pointer to make sure the uninitialized access panics

func GetGiteaTestSourceRoot() string {
	return *giteaTestSourceRoot
}

func detectGiteaTestRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	giteaRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	fixturesDir := filepath.Join(giteaRoot, "models", "fixtures")
	if _, err := os.Stat(fixturesDir); err != nil {
		panic("in gitea source code directory, fixtures directory not found: " + fixturesDir)
	}
	return giteaRoot
}

func SetupGiteaTestEnv() {
	if giteaTestSourceRoot != nil {
		return // already initialized
	}

	IsInTesting = true

	log.OsExiter = func(code int) {
		if code != 0 {
			// Non-zero exit code (log.Fatal) shouldn't occur during testing, if it happens:
			// * Show a full stacktrace for more details.
			// * If the "log.Fatal" is abused in tests, should fix.
			panic(fmt.Errorf("non-zero exit code during testing: %d", code))
		}
		os.Exit(0)
	}

	initGiteaRoot := func() string {
		giteaRoot := os.Getenv("GITEA_TEST_ROOT")
		if giteaRoot == "" {
			giteaRoot = detectGiteaTestRoot()
		}
		giteaTestSourceRoot = &giteaRoot
		return giteaRoot
	}
	giteaRoot := initGiteaRoot()

	initGiteaPaths := func() {
		// need to load assets (options, public) from the source code directory for testing
		StaticRootPath = giteaRoot
		// during testing, the AppPath must point to the pre-built Gitea binary in the source root
		// it needs to be called by git hooks
		AppPath = filepath.Join(giteaRoot, "gitea") + util.Iif(IsWindows, ".exe", "")
	}

	initGiteaConf := func() string {
		// giteaConf (GITEA_CONF) must be relative because it is used in the git hooks as "$GITEA_ROOT/$GITEA_CONF"
		giteaConf := os.Getenv("GITEA_TEST_CONF")
		if giteaConf == "" {
			// if no GITEA_TEST_CONF, then it is in unit test, use a temp (non-existing / empty) config file
			// do not really use such config file, the test can run concurrently, using the same config file will cause data-race between tests
			giteaConf = "custom/conf/app-test-tmp.ini"
			customConfBuiltin = filepath.Join(AppWorkPath, giteaConf)
			CustomConf = customConfBuiltin
			_ = os.Remove(CustomConf)
		} else {
			// CustomConf must be absolute path to make tests pass.
			// At the moment, GITEA_TEST_CONF is always in Gitea's source root
			CustomConf = filepath.Join(giteaRoot, giteaConf)
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

	initGiteaPaths()
	giteaConf := initGiteaConf()
	cleanUpEnv()
	initWorkPathAndConfig()

	if RepoRootPath == "" || AppDataPath == "" {
		panic("SetupGiteaTestEnv failed, paths are not initialized")
	}

	// TODO: some git repo hooks (test fixtures) still use these env variables, need to be refactored in the future
	_ = os.Setenv("GITEA_ROOT", giteaRoot)
	_ = os.Setenv("GITEA_CONF", giteaConf) // test fixture git hooks use "$GITEA_ROOT/$GITEA_CONF" in their scripts
}

func PrepareIntegrationTestConfig() error {
	giteaTestRoot := detectGiteaTestRoot()
	isInCI := os.Getenv("CI") != ""
	testDatabase := os.Getenv("GITEA_TEST_DATABASE")
	if testDatabase == "" {
		if isInCI {
			return errors.New("GITEA_TEST_DATABASE environment variable not set")
		}
		// for local development, default to sqlite. CI needs to explicitly set a database to avoid unexpected results
		testDatabase = "sqlite"
		_, _ = fmt.Fprintf(os.Stderr, "Environment variable GITEA_TEST_DATABASE not set - defaulting to %s\n", testDatabase)
	}

	_ = os.Setenv("GITEA_TEST_ROOT", giteaTestRoot)
	_ = os.Setenv("GITEA_TEST_CONF", filepath.Join("tests", testDatabase+".ini"))

	workPath := filepath.Join(giteaTestRoot, "tests/integration/gitea-integration-"+testDatabase)
	if err := os.MkdirAll(workPath, 0o755); err != nil {
		return err
	}

	confFile := filepath.Join(giteaTestRoot, "tests", testDatabase+".ini")
	tmplBuf, err := os.ReadFile(confFile + ".tmpl")
	if err != nil {
		return err
	}
	tmpl := string(tmplBuf)
	envVars, err := shellquote.Split(os.Getenv("MAKEFILE_VARS"))
	if err != nil {
		return err
	}
	envVarMap := map[string]string{
		"TEST_WORK_PATH": workPath,
		"TEST_LOGGER":    "test,file",
	}
	for _, env := range append(os.Environ(), envVars...) {
		k, v, _ := strings.Cut(env, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		envVarMap[k] = v
	}
	for k, v := range envVarMap {
		tmpl = strings.ReplaceAll(tmpl, fmt.Sprintf("{{%s}}", k), v)
	}
	err = os.WriteFile(confFile, []byte(tmpl), 0o644)
	return err
}
