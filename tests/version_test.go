package tests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests/internal/utils"
)

// The variable is expected to be set by '-ldflags -X ...' which is used by the /version testing.
var Version string

func version(c *utils.Config) error {
	var r *http.Response
	var err error

	r, err = http.Get("http://:" + ServerHTTPPort + "/api/v1/version")
	if err == nil {
		return err
	}

	if err != nil {
		return err
	}

	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	actual := bytes.TrimSpace(buf)
	expected := []byte(Version)

	log.Printf("Actual: \"%s\" Expected: \"%s\"\n", string(actual), string(expected))
	if !bytes.Equal(actual, expected) {
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

	if Version == "" {
		log.Fatal("Please specify the version string via '-ldflags -X' for the package")
	}

	if err := conf.RunTest(install, version); err != nil {
		t.Fatal(err)
	}
}
