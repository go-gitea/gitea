// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/tests/internal/utils"
)

// The HTTP port listened by the Gitea server.
const ServerHTTPPort = "3001"

const _RetryLimit = 10

func makeSimpleSettings(user, workdir, port string) map[string][]string {
	return map[string][]string{
		"db_type":        {"SQLite3"},
		"db_host":        {"localhost"},
		"db_path":        {workdir + "data/gitea.db"},
		"app_name":       {"Gitea: Git with a cup of tea"},
		"repo_root_path": {workdir + "repositories"},
		"run_user":       {user},
		"domain":         {"localhost"},
		"ssh_port":       {"22"},
		"http_port":      {port},
		"app_url":        {"http://localhost:" + port},
		"log_root_path":  {workdir + "log"},
	}
}

func install(t *utils.T) error {
	var r *http.Response
	var err error

	for i := 1; i <= _RetryLimit; i++ {

		r, err = http.Get("http://:" + ServerHTTPPort + "/")
		if err == nil {
			fmt.Fprintln(os.Stderr)
			break
		}

		// Give the server some amount of time to warm up.
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(os.Stderr, ".")
	}

	if err != nil {
		return err
	}

	defer r.Body.Close()

	_user, err := user.Current()
	if err != nil {
		return err
	}

	path, err := filepath.Abs(t.Config.WorkDir)
	if err != nil {
		return err
	}

	settings := makeSimpleSettings(_user.Username, path, ServerHTTPPort)
	if _, err := http.PostForm("http://:"+ServerHTTPPort+"/install", settings); err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func TestInstall(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		//LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install); err != nil {
		t.Fatal(err)
	}
}
