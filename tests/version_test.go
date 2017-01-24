package tests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/tests/internal/utils"
)

const RetryLimit = 10

var Version string

func version(c *utils.Config) error {
	var r *http.Response
	var err error

	for i := 0; i < RetryLimit; i++ {
		r, err = http.Get("http://:3001/api/v1/version")
		if err == nil {
			break
		}

		// Give the server some amount of time to warm up.
		fmt.Fprintf(os.Stderr, "Retry %d\n", i)
		time.Sleep(500 * time.Millisecond)
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
		return fmt.Errorf("Do not match!")
	}
	return nil
}

func TestVersion(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", "3001"},
		//LogFile: os.Stderr,
	}

	if Version == "" {
		log.Fatal("Please specify the version string via '-ldflags -X' for the package")
	}

	if err := conf.RunTest(install, version); err != nil {
		t.Fatal(err)
	}
}
