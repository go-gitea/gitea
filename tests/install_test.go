package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/tests/internal/utils"
)

const _RetryLimit = 10

//"db_user":        {},
//"db_passwd":      {},
//"db_name":        {},

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

func TestInstall(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", "3001"},
		//LogFile: os.Stderr,
	}

	if err := conf.RunTest(func() error {
		var r *http.Response
		var err error

		for i := 0; i < _RetryLimit; i++ {
			// Give the server some amount of time to warm up.
			time.Sleep(500 * time.Millisecond)

			r, err = http.Get("http://:3001/")
			if err == nil {
				break
			}
			fmt.Fprintf(os.Stderr, "Retry %d\n", i)
		}

		if err != nil {
			return err
		}

		defer r.Body.Close()

		_user, err := user.Current()
		if err != nil {
			return err
		}

		path, err := filepath.Abs(conf.WorkDir)
		if err != nil {
			return err
		}

		settings := makeSimpleSettings(_user.Username, path, "3001")
		resp, err := http.PostForm("http://:3001/install", settings)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", buf)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
