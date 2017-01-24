package tests

import (
	"bytes"
	"errors"
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
		// Give the server some amount of time to warm up.
		time.Sleep(500 * time.Millisecond)

		r, err = http.Get("http://:3001/api/v1/version")
		if err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "Retry %d\n", i)
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

	log.Printf("Actual: \"%s\"\n", string(actual))
	log.Printf("Expected: \"%s\"\n", string(expected))
	if !bytes.Equal(actual, expected) {
		return errors.New(fmt.Sprintf("Do not match!"))
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
		fmt.Fprintf(os.Stderr, "ADDE")
		t.Fatal(err)
	}
}
