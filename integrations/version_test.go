// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/integrations/internal/utils"
	"code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func version(t *utils.T) error {
	var err error

	path, err := filepath.Abs(t.Config.Program)
	if err != nil {
		return err
	}

	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	fields := strings.Fields(string(out))
	if !strings.HasPrefix(string(out), "Gitea version") {
		return fmt.Errorf("unexpected version string '%s' of the gitea executable", out)
	}

	expected := fields[2]

	var r *http.Response
	r, err = http.Get("http://:" + ServerHTTPPort + "/api/v1/version")
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("'/api/v1/version': %s\n", r.Status)
	}

	var v gitea.ServerVersion

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&v); err != nil {
		return err
	}

	actual := v.Version

	log.Printf("Actual: \"%s\" Expected: \"%s\"\n", actual, expected)
	assert.Equal(t, expected, actual)

	return nil
}

func TestVersion(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install, version); err != nil {
		t.Fatal(err)
	}
}
