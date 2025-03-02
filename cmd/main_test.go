// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func makePathOutput(workPath, customPath, customConf string) string {
	return fmt.Sprintf("WorkPath=%s\nCustomPath=%s\nCustomConf=%s", workPath, customPath, customConf)
}

func newTestApp(testCmdAction func(ctx *cli.Context) error) *cli.App {
	app := NewMainApp(AppVersion{})
	testCmd := &cli.Command{Name: "test-cmd", Action: testCmdAction}
	prepareSubcommandWithConfig(testCmd, appGlobalFlags())
	app.Commands = append(app.Commands, testCmd)
	app.DefaultCommand = testCmd.Name
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

func TestCliCmd(t *testing.T) {
	defaultWorkPath := filepath.Dir(setting.AppPath)
	defaultCustomPath := filepath.Join(defaultWorkPath, "custom")
	defaultCustomConf := filepath.Join(defaultCustomPath, "conf/app.ini")

	cli.CommandHelpTemplate = "(command help template)"
	cli.AppHelpTemplate = "(app help template)"
	cli.SubcommandHelpTemplate = "(subcommand help template)"

	cases := []struct {
		env map[string]string
		cmd string
		exp string
	}{
		// main command help
		{
			cmd: "./gitea help",
			exp: "DEFAULT CONFIGURATION:",
		},

		// parse paths
		{
			cmd: "./gitea test-cmd",
			exp: makePathOutput(defaultWorkPath, defaultCustomPath, defaultCustomConf),
		},
		{
			cmd: "./gitea -c /tmp/app.ini test-cmd",
			exp: makePathOutput(defaultWorkPath, defaultCustomPath, "/tmp/app.ini"),
		},
		{
			cmd: "./gitea test-cmd -c /tmp/app.ini",
			exp: makePathOutput(defaultWorkPath, defaultCustomPath, "/tmp/app.ini"),
		},
		{
			env: map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmd: "./gitea test-cmd",
			exp: makePathOutput("/tmp", "/tmp/custom", "/tmp/custom/conf/app.ini"),
		},
		{
			env: map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmd: "./gitea test-cmd --work-path /tmp/other",
			exp: makePathOutput("/tmp/other", "/tmp/other/custom", "/tmp/other/custom/conf/app.ini"),
		},
		{
			env: map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmd: "./gitea test-cmd --config /tmp/app-other.ini",
			exp: makePathOutput("/tmp", "/tmp/custom", "/tmp/app-other.ini"),
		},
	}

	app := newTestApp(func(ctx *cli.Context) error {
		_, _ = fmt.Fprint(ctx.App.Writer, makePathOutput(setting.AppWorkPath, setting.CustomPath, setting.CustomConf))
		return nil
	})
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			args := strings.Split(c.cmd, " ") // for test only, "split" is good enough
			r, err := runTestApp(app, args...)
			assert.NoError(t, err, c.cmd)
			assert.NotEmpty(t, c.exp, c.cmd)
			assert.Contains(t, r.Stdout, c.exp, c.cmd)
		})
	}
}

func TestCliCmdError(t *testing.T) {
	app := newTestApp(func(ctx *cli.Context) error { return fmt.Errorf("normal error") })
	r, err := runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "Command error: normal error\n", r.Stderr)

	app = newTestApp(func(ctx *cli.Context) error { return cli.Exit("exit error", 2) })
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 2, r.ExitCode)
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "exit error\n", r.Stderr)

	app = newTestApp(func(ctx *cli.Context) error { return nil })
	r, err = runTestApp(app, "./gitea", "test-cmd", "--no-such")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.Equal(t, "Incorrect Usage: flag provided but not defined: -no-such\n\n", r.Stdout)
	assert.Equal(t, "", r.Stderr) // the cli package's strange behavior, the error message is not in stderr ....

	app = newTestApp(func(ctx *cli.Context) error { return nil })
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.NoError(t, err)
	assert.Equal(t, -1, r.ExitCode) // the cli.OsExiter is not called
	assert.Equal(t, "", r.Stdout)
	assert.Equal(t, "", r.Stderr)
}
