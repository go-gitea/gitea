package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type Config struct {
	// The executable path of the tested program.
	Program string
	// Working directory prepared for the tested program.
	// If empty, a directory named with random suffixes is picked, and created under the current directory.
	// The directory will be removed when the test finishes.
	WorkDir string
	// Command-line arguments passed to the tested program.
	Args []string
}

// Helper function for setting up a running Gitea server for functional testing and then gracefully terminating it.
func (c *Config) RunTest(testFunc func() error) error {
	if c.Program == "" {
		return errors.New("Need input file")
	}

	workdir := c.WorkDir
	if workdir == "" {
		workdir = fmt.Sprintf("gitea-%s-%10d", filepath.Base(c.Program), time.Now().UnixNano())
		if err := os.Mkdir(workdir, 0700); err != nil {
			return err
		}
		defer os.Remove(workdir)
	}

	fullpath, err := filepath.Abs(c.Program)
	if err != nil {
		return err
	}

	cmd := exec.Command(fullpath, c.Args...)
	cmd.Dir = workdir
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := testFunc(); err != nil {
		return err
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if err.Error() != "signal: terminated" {
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "Passed")
	return nil
}
