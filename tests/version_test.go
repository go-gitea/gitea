package tests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/tests/internal/utils"
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
	if len(fields) != 3 {
		return fmt.Errorf("unexpected version string")
	}

	expected := fields[2]

	var r *http.Response
	r, err = http.Get("http://:" + ServerHTTPPort + "/api/v1/version")
	if err == nil {
		return err
	}

	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	actual := string(bytes.TrimSpace(buf))

	log.Printf("Actual: \"%s\" Expected: \"%s\"\n", string(actual), string(expected))
	if actual != expected {
		return fmt.Errorf("do not match")
	}
	return nil
}

func TestVersion(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", ServerHTTPPort},
		//LogFile: os.Stderr,
	}

	if err := utils.New(t, &conf).RunTest(install, version); err != nil {
		t.Fatal(err)
	}
}
