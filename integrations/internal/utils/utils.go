// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// T wraps testing.T and the configurations of the testing instance.
type T struct {
	*testing.T
	Config *Config
}

// New create an instance of T
func New(t *testing.T, c *Config) *T {
	return &T{T: t, Config: c}
}

// Config Settings of the testing program
type Config struct {
	// The executable path of the tested program.
	Program string
	// Working directory prepared for the tested program.
	// If empty, a directory named with random suffixes is picked, and created under the platform-dependent default temporary directory.
	// The directory will be removed when the test finishes.
	WorkDir string
	// Command-line arguments passed to the tested program.
	Args []string

	// Where to redirect the stdout/stderr to. For debugging purposes.
	LogFile *os.File
}

func redirect(cmd *exec.Cmd, f *os.File) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go io.Copy(f, stdout)
	go io.Copy(f, stderr)
	return nil
}

// RunTest Helper function for setting up a running Gitea server for functional testing and then gracefully terminating it.
func (t *T) RunTest(tests ...func(*T) error) (err error) {
	if t.Config.Program == "" {
		return errors.New("Need input file")
	}

	path, err := filepath.Abs(t.Config.Program)
	if err != nil {
		return err
	}

	workdir := t.Config.WorkDir
	if workdir == "" {
		workdir, err = ioutil.TempDir(os.TempDir(), "gitea_tests-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(workdir)
	}

	newpath := filepath.Join(workdir, filepath.Base(path))
	if err := os.Symlink(path, newpath); err != nil {
		return err
	}

	log.Printf("Starting the server: %s args:%s workdir:%s", newpath, t.Config.Args, workdir)

	cmd := exec.Command(newpath, t.Config.Args...)
	cmd.Dir = workdir

	if t.Config.LogFile != nil && testing.Verbose() {
		if err := redirect(cmd, t.Config.LogFile); err != nil {
			return err
		}
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	log.Println("Server started.")

	defer func() {
		// Do not early return. We have to call Wait anyway.
		_ = cmd.Process.Signal(syscall.SIGTERM)

		if _err := cmd.Wait(); _err != nil {
			if _err.Error() != "signal: terminated" {
				err = _err
				return
			}
		}

		log.Println("Server exited")
	}()

	for _, fn := range tests {
		if err := fn(t); err != nil {
			return err
		}
	}

	// Note that the return value 'err' may be updated by the 'defer' statement before despite it's returning nil here.
	return nil
}

// GetAndPost provides a convenient helper function for testing an HTTP endpoint with GET and POST method.
// The function sends GET first and then POST with the given form.
func GetAndPost(url string, form map[string][]string) error {
	var err error
	var r *http.Response

	r, err = http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("GET '%s': %s", url, r.Status)
	}

	r, err = http.PostForm(url, form)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("POST '%s': %s", url, r.Status)
	}

	return nil
}
