// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"codenest.dev/modules/auth/password/hash"
	"codenest.dev/modules/consts"
	"codenest.dev/modules/log"
	"codenest.dev/modules/util"

	"github.com/kballard/go-shellquote"
)

var codenestTestSourceRoot *string // intentionally use a pointer to make sure the uninitialized access panics

func GetCodeNestTestSourceRoot() string {
	return *codenestTestSourceRoot
}

func detectCodeNestTestRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	codenestRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	fixturesDir := filepath.Join(codenestRoot, "models", "fixtures")
	if _, err := os.Stat(fixturesDir); err != nil {
		panic("in codenest source code directory, fixtures directory not found: " + fixturesDir)
	}
	return codenestRoot
}

func SetupCodeNestTestEnv() {
	if codenestTestSourceRoot != nil {
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

	initCodeNestRoot := func() string {
		codenestRoot := os.Getenv("CODENEST_TEST_ROOT")
		if codenestRoot == "" {
			codenestRoot = detectCodeNestTestRoot()
		}
		codenestTestSourceRoot = &codenestRoot
		return codenestRoot
	}
	codenestRoot := initCodeNestRoot()

	initCodeNestPaths := func() {
		// need to load assets (options, public) from the source code directory for testing
		StaticRootPath = codenestRoot
		// during testing, the AppPath must point to the pre-built CodeNest binary in the source root
		// it needs to be called by git hooks
		AppPath = filepath.Join(codenestRoot, "codenest") + util.Iif(consts.IsWindows, ".exe", "")
	}

	initCodeNestConf := func() string {
		// codenestConf (CODENEST_CONF) must be relative because it is used in the git hooks as "$CODENEST_ROOT/$CODENEST_CONF"
		codenestConf := os.Getenv("CODENEST_TEST_CONF")
		if codenestConf == "" {
			// if no CODENEST_TEST_CONF, then it is in unit test, use a temp (non-existing / empty) config file
			// do not really use such config file, the test can run concurrently, using the same config file will cause data-race between tests
			codenestConf = "custom/conf/app-test-tmp.ini"
			customConfBuiltin = filepath.Join(AppWorkPath, codenestConf)
			CustomConf = customConfBuiltin
			_ = os.Remove(CustomConf)
		} else {
			// CustomConf must be absolute path to make tests pass.
			// At the moment, CODENEST_TEST_CONF is always in CodeNest's source root
			CustomConf = filepath.Join(codenestRoot, codenestConf)
		}
		return codenestConf
	}

	cleanUpEnv := func() {
		// also unset unnecessary env vars for testing (only keep "CODENEST_TEST_*" ones)
		UnsetUnnecessaryEnvVars()
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "GIT_") || (strings.HasPrefix(env, "CODENEST_") && !strings.HasPrefix(env, "CODENEST_TEST_")) {
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

	initCodeNestPaths()
	codenestConf := initCodeNestConf()
	cleanUpEnv()
	initWorkPathAndConfig()

	if RepoRootPath == "" || AppDataPath == "" {
		panic("SetupCodeNestTestEnv failed, paths are not initialized")
	}

	SSH.RootPath = AppDataTempDir("ssh").JoinPath() // prevent tests from writing into ~/.ssh

	// TODO: some git repo hooks (test fixtures) still use these env variables, need to be refactored in the future
	_ = os.Setenv("CODENEST_ROOT", codenestRoot)
	_ = os.Setenv("CODENEST_CONF", codenestConf) // test fixture git hooks use "$CODENEST_ROOT/$CODENEST_CONF" in their scripts
}

// PrepareIntegrationTestConfig isolates each process for parallel runs
func PrepareIntegrationTestConfig() (workPath string, err error) {
	codenestTestRoot := detectCodeNestTestRoot()
	isInCI := os.Getenv("CI") != ""
	testDatabase := os.Getenv("CODENEST_TEST_DATABASE")
	if testDatabase == "" {
		if isInCI {
			return "", errors.New("CODENEST_TEST_DATABASE environment variable not set")
		}
		// for local development, default to sqlite. CI needs to explicitly set a database to avoid unexpected results
		testDatabase = "sqlite"
		_, _ = fmt.Fprintf(os.Stderr, "Environment variable CODENEST_TEST_DATABASE not set - defaulting to %s\n", testDatabase)
	}

	workPath, err = os.MkdirTemp(filepath.Join(codenestTestRoot, "tests/integration"), "codenest-integration-"+testDatabase+"-")
	if err != nil {
		return "", err
	}
	confFile := filepath.Join(workPath, "app.ini")
	codenestConf, err := filepath.Rel(codenestTestRoot, confFile)
	if err != nil {
		return "", err
	}
	_ = os.Setenv("CODENEST_TEST_ROOT", codenestTestRoot)
	_ = os.Setenv("CODENEST_TEST_CONF", codenestConf)

	tmplBuf, err := os.ReadFile(filepath.Join(codenestTestRoot, "tests", testDatabase+".ini.tmpl"))
	if err != nil {
		return "", err
	}
	tmpl := string(tmplBuf)
	envVars, err := shellquote.Split(os.Getenv("MAKEFILE_VARS"))
	if err != nil {
		return "", err
	}
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer httpListener.Close()
	sshListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer sshListener.Close()
	_, httpPort, _ := net.SplitHostPort(httpListener.Addr().String())
	_, sshPort, _ := net.SplitHostPort(sshListener.Addr().String())
	envVarMap := map[string]string{
		"TEST_WORK_PATH": workPath,
		"TEST_LOGGER":    "test,file",
		"TEST_HTTP_PORT": httpPort,
		"TEST_SSH_PORT":  sshPort,
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
	return workPath, os.WriteFile(confFile, []byte(tmpl), 0o644)
}
