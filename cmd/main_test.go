// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: "..",
	})
}

func newTestApp(testCmdAction func(ctx *cli.Context) error) *cli.App {
	app := cli.NewApp()
	app.HelpName = "gitea"
	testCmd := cli.Command{Name: "test-cmd", Action: testCmdAction}
	app.Commands = append(app.Commands, testCmd)
	return app
}

type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func runTestApp(app *cli.App, args ...string) (runResult, error) {
	outBuf := new(strings.Builder)
	errBuf := new(strings.Builder)
	app.Writer = outBuf
	app.ErrWriter = errBuf
	exitCode := -1
	defer test.MockVariableValue(&cli.ErrWriter, app.ErrWriter)()
	defer test.MockVariableValue(&cli.OsExiter, func(code int) {
		if exitCode == -1 {
			exitCode = code // save the exit code once and then reset the writer (to simulate the exit)
			app.Writer, app.ErrWriter, cli.ErrWriter = io.Discard, io.Discard, io.Discard
		}
	})()
	err := RunMainApp(app, args...)
	return runResult{outBuf.String(), errBuf.String(), exitCode}, err
}

func TestCliCmdError(t *testing.T) {
	app := newTestApp(func(ctx *cli.Context) error { return fmt.Errorf("normal error") })
	r, err := runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "Command error: normal error\n", r.Stderr)

	app = newTestApp(func(ctx *cli.Context) error { return cli.NewExitError("exit error", 2) })
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 2, r.ExitCode)
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "exit error\n", r.Stderr)

	app = newTestApp(func(ctx *cli.Context) error { return nil })
	r, err = runTestApp(app, "./gitea", "test-cmd", "--no-such")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.EqualValues(t, "Incorrect Usage: flag provided but not defined: -no-such\n\nNAME:\n   gitea test-cmd - \n\nUSAGE:\n   gitea test-cmd [arguments...]\n", r.Stdout)
	assert.Equal(t, "", r.Stderr) // the cli package's strange behavior, the error message is not in stderr ....

	app = newTestApp(func(ctx *cli.Context) error { return nil })
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.NoError(t, err)
	assert.Equal(t, -1, r.ExitCode) // the cli.OsExiter is not called
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "", r.Stderr)
}
