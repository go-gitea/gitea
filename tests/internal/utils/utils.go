package utils

import (
	"errors"
	"fmt"
	"io"
	"log"
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

	go io.Copy(os.Stderr, stdout)
	go io.Copy(os.Stdout, stderr)
	return nil
}

// Helper function for setting up a running Gitea server for functional testing and then gracefully terminating it.
func (c *Config) RunTest(tests ...func(*Config) error) (err error) {
	if c.Program == "" {
		return errors.New("Need input file")
	}

	path, err := filepath.Abs(c.Program)
	if err != nil {
		return err
	}

	workdir := c.WorkDir
	if workdir == "" {
		workdir, err = filepath.Abs(fmt.Sprintf("%s-%10d", filepath.Base(c.Program), time.Now().UnixNano()))
		if err != nil {
			return err
		}
		if err := os.Mkdir(workdir, 0700); err != nil {
			return err
		}
		defer os.RemoveAll(workdir)
	}

	newpath := filepath.Join(workdir, filepath.Base(path))
	if err := os.Link(path, newpath); err != nil {
		return err
	}

	log.Printf("Starting the server: %s args:%s workdir:%s", newpath, c.Args, workdir)

	cmd := exec.Command(newpath, c.Args...)
	cmd.Dir = workdir

	if c.LogFile != nil {
		if err := redirect(cmd, c.LogFile); err != nil {
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
		if err := fn(c); err != nil {
			return err
		}
	}

	return nil
}
