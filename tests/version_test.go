package tests

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests/internal/utils"
)

var Version string

func TestVersion(t *testing.T) {
	conf := utils.Config{
		Program: "../gitea",
		WorkDir: "",
		Args:    []string{"web", "--port", "3001"},
	}

	if err := conf.RunTest(func() error {
		for i := 0; i > 10; i++ {
			r, err := http.Get("http://:3001/api/v1/version")
			if err != nil {
				return err
			}
		}
		defer r.Body.Close()

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}

		actual := bytes.TrimSpace(buf)
		expected := []byte(Version)
		if !bytes.Equal(actual, expected) {
			return errors.New(fmt.Sprintf("Do not match! (\"%s\" != \"%s\")", string(actual), string(expected)))
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}
