// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests here reload the config system multiple times with uncontrollable details.
// So they must be in a separate package, to avoid affecting other tests

package cmdtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/cmd"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func makePathOutput(workPath, customPath, customConf string) string {
	return fmt.Sprintf("WorkPath=%s\nCustomPath=%s\nCustomConf=%s", workPath, customPath, customConf)
}

func newTestApp(testCmd cli.Command) *cli.Command {
	app := cmd.NewMainApp(cmd.AppVersion{})
	testCmd.Name = util.IfZero(testCmd.Name, "test-cmd")
	cmd.PrepareSubcommandWithGlobalFlags(&testCmd)
	app.Commands = append(app.Commands, &testCmd)
	app.DefaultCommand = testCmd.Name
	return app
}

type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func runTestApp(app *cli.Command, args ...string) (runResult, error) {
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
	err := cmd.RunMainApp(app, args...)
	return runResult{outBuf.String(), errBuf.String(), exitCode}, err
}

func TestCliCmd(t *testing.T) {
	defaultWorkPath := filepath.FromSlash("/tmp/mocked-work-path")
	defaultCustomPath := filepath.Join(defaultWorkPath, "custom")
	defaultCustomConf := filepath.Join(defaultCustomPath, "conf/app.ini")
	defer setting.MockBuiltinPaths(defaultWorkPath, "", "")()

	cli.CommandHelpTemplate = "(command help template)"
	cli.RootCommandHelpTemplate = "(app help template)"
	cli.SubcommandHelpTemplate = "(subcommand help template)"

	cases := []struct {
		env  map[string]string
		cmds []string
		exp  string
	}{
		// help commands
		{
			cmds: []string{"./gitea -h", "./gitea help"},
			exp:  "DEFAULT CONFIGURATION:",
		},
		{
			cmds: []string{
				"./gitea -c /dev/null -h",
				"./gitea -c /dev/null help",
				"./gitea help -c /dev/null",
				"./gitea -c /dev/null test-cmd -h",
				"./gitea test-cmd -c /dev/null -h",
				"./gitea test-cmd -h -c /dev/null",
				"./gitea -c /dev/null test-cmd help",
				"./gitea test-cmd -c /dev/null help",
				"./gitea test-cmd help -c /dev/null",
			},
			exp: "ConfigFile: /dev/null",
		},

		// parse paths
		{
			cmds: []string{"./gitea test-cmd"},
			exp:  makePathOutput(defaultWorkPath, defaultCustomPath, defaultCustomConf),
		},
		{
			cmds: []string{"./gitea -c /tmp/app.ini test-cmd"},
			exp:  makePathOutput(defaultWorkPath, defaultCustomPath, "/tmp/app.ini"),
		},
		{
			cmds: []string{"./gitea test-cmd -c /tmp/app.ini"},
			exp:  makePathOutput(defaultWorkPath, defaultCustomPath, "/tmp/app.ini"),
		},
		{
			env:  map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmds: []string{"./gitea test-cmd"},
			exp:  makePathOutput("/tmp", "/tmp/custom", "/tmp/custom/conf/app.ini"),
		},
		{
			env:  map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmds: []string{"./gitea test-cmd --work-path /tmp/other"},
			exp:  makePathOutput("/tmp/other", "/tmp/other/custom", "/tmp/other/custom/conf/app.ini"),
		},
		{
			env:  map[string]string{"GITEA_WORK_DIR": "/tmp"},
			cmds: []string{"./gitea test-cmd --config /tmp/app-other.ini"},
			exp:  makePathOutput("/tmp", "/tmp/custom", "/tmp/app-other.ini"),
		},
	}

	for _, c := range cases {
		for _, command := range c.cmds {
			t.Run(command, func(t *testing.T) {
				app := newTestApp(cli.Command{
					Action: func(ctx context.Context, cmd *cli.Command) error {
						_, _ = fmt.Fprint(cmd.Root().Writer, makePathOutput(setting.AppWorkPath, setting.CustomPath, setting.CustomConf))
						return nil
					},
				})
				for k, v := range c.env {
					t.Setenv(k, v)
				}
				args := strings.Split(command, " ") // for test only, "split" is good enough
				r, err := runTestApp(app, args...)
				assert.NoError(t, err, command)
				assert.NotEmpty(t, c.exp, command)
				if !assert.Contains(t, r.Stdout, c.exp, command) {
					t.Log("Full output:\n" + r.Stdout)
					t.Log("Expected:\n" + c.exp)
				}
			})
		}
	}
}

func TestCliCmdError(t *testing.T) {
	app := newTestApp(cli.Command{Action: func(ctx context.Context, cmd *cli.Command) error { return errors.New("normal error") }})
	r, err := runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.Empty(t, r.Stdout)
	assert.Equal(t, "Command error: normal error\n", r.Stderr)

	app = newTestApp(cli.Command{Action: func(ctx context.Context, cmd *cli.Command) error { return cli.Exit("exit error", 2) }})
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.Error(t, err)
	assert.Equal(t, 2, r.ExitCode)
	assert.Empty(t, r.Stdout)
	assert.Equal(t, "exit error\n", r.Stderr)

	app = newTestApp(cli.Command{Action: func(ctx context.Context, cmd *cli.Command) error { return nil }})
	r, err = runTestApp(app, "./gitea", "test-cmd", "--no-such")
	assert.Error(t, err)
	assert.Equal(t, 1, r.ExitCode)
	assert.Empty(t, r.Stdout)
	assert.Equal(t, "Incorrect Usage: flag provided but not defined: -no-such\n", r.Stderr)

	app = newTestApp(cli.Command{Action: func(ctx context.Context, cmd *cli.Command) error { return nil }})
	r, err = runTestApp(app, "./gitea", "test-cmd")
	assert.NoError(t, err)
	assert.Equal(t, -1, r.ExitCode) // the cli.OsExiter is not called
	assert.Empty(t, r.Stdout)
	assert.Empty(t, r.Stderr)
}

func TestCliCmdBefore(t *testing.T) {
	ctxNew := context.WithValue(context.Background(), any("key"), "value")
	configValues := map[string]string{}
	setting.CustomConf = "/tmp/any.ini"
	var actionCtx context.Context
	app := newTestApp(cli.Command{
		Before: func(context.Context, *cli.Command) (context.Context, error) {
			configValues["before"] = setting.CustomConf
			return ctxNew, nil
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			configValues["action"] = setting.CustomConf
			actionCtx = ctx
			return nil
		},
	})
	_, err := runTestApp(app, "./gitea", "--config", "/dev/null", "test-cmd")
	assert.NoError(t, err)
	assert.Equal(t, ctxNew, actionCtx)
	assert.Equal(t, "/tmp/any.ini", configValues["before"], "BeforeFunc must be called before preparing config")
	assert.Equal(t, "/dev/null", configValues["action"])
}

func TestCliCmdCompletion(t *testing.T) {
	app := newTestApp(cli.Command{
		Action: func(ctx context.Context, cmd *cli.Command) error { return nil },
	})
	res, err := runTestApp(app, "./gitea", "completion", "bash", "--nonexist")
	assert.Error(t, err)
	assert.Equal(t, 1, res.ExitCode)
	assert.Equal(t, "", res.Stdout)
	assert.Equal(t, "Incorrect Usage: flag provided but not defined: -nonexist\n", res.Stderr)
}
