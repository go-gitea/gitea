// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Command represents a command with its subcommands or arguments.
type Command struct {
	name string
	args []string
}

func (c *Command) String() string {
	if len(c.args) == 0 {
		return c.name
	}
	return fmt.Sprintf("%s %s", c.name, strings.Join(c.args, " "))
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
func NewCommand(args ...string) *Command {
	return &Command{
		name: "git",
		args: args,
	}
}

// AddArguments adds new argument(s) to the command.
func (c *Command) AddArguments(args ...string) *Command {
	c.args = append(c.args, args...)
	return c
}

func (c *Command) run(dir string) (string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := exec.Command(c.name, c.args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if len(dir) == 0 {
		log(c.String())
	} else {
		log("%s: %v", dir, c)
	}

	if err := cmd.Run(); err != nil {
		return stdout.String(), concatenateError(err, stderr.String())
	}
	if stdout.Len() > 0 {
		log("stdout:\n%s", stdout)
	}

	return stdout.String(), nil
}

// Run executes the command in defualt working directory
// and returns stdout and error (combined with stderr).
func (c *Command) Run() (string, error) {
	return c.run("")
}

// RunInDir executes the command in given directory
// and returns stdout and error (combined with stderr).
func (c *Command) RunInDir(dir string) (string, error) {
	return c.run(dir)
}
