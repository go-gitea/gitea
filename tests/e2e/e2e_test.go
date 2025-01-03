// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This is primarily coped from /tests/integration/integration_test.go
//   TODO: Move common functions to shared file

//nolint:forbidigo
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/tests"
)

var testE2eWebRoutes *web.Router

func TestMain(m *testing.M) {
	defer log.GetManager().Close()

	managerCtx, cancel := context.WithCancel(context.Background())
	graceful.InitManager(managerCtx)
	defer cancel()

	tests.InitTest(false)
	testE2eWebRoutes = routers.NormalRoutes()

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

	testlogger.WriterCloser.Reset()

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

// TestE2e should be the only test e2e necessary. It will collect all "*.test.e2e.ts" files in this directory and build a test for each.
func TestE2e(t *testing.T) {
	// Find the paths of all e2e test files in test directory.
	searchGlob := filepath.Join(filepath.Dir(setting.AppPath), "tests", "e2e", "*.test.e2e.ts")
	paths, err := filepath.Glob(searchGlob)
	if err != nil {
		t.Fatal(err)
	} else if len(paths) == 0 {
		t.Fatal(fmt.Errorf("No e2e tests found in %s", searchGlob))
	}

	runArgs := []string{"npx", "playwright", "test"}

	// To update snapshot outputs
	if _, set := os.LookupEnv("ACCEPT_VISUAL"); set {
		runArgs = append(runArgs, "--update-snapshots")
	}

	// Create new test for each input file
	for _, path := range paths {
		_, filename := filepath.Split(path)
		testname := filename[:len(filename)-len(filepath.Ext(path))]

		t.Run(testname, func(t *testing.T) {
			// Default 2 minute timeout
			onGiteaRun(t, func(*testing.T, *url.URL) {
				cmd := exec.Command(runArgs[0], runArgs...)
				cmd.Env = os.Environ()
				cmd.Env = append(cmd.Env, fmt.Sprintf("GITEA_URL=%s", setting.AppURL))

				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr

				err = cmd.Run()
				if err != nil {
					// Currently colored output is conflicting. Using Printf until that is resolved.
					fmt.Printf("%v", stdout.String())
					fmt.Printf("%v", stderr.String())
					log.Fatal("Playwright Failed: %s", err)
				}

				fmt.Printf("%v", stdout.String())
			})
		})
	}
}
