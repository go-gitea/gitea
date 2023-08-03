// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: "..",
	})
}

func makePathOutput(workPath, customPath, customConf string) string {
	return fmt.Sprintf("WorkPath=%s\nCustomPath=%s\nCustomConf=%s", workPath, customPath, customConf)
}

func newTestApp() *cli.App {
	app := NewMainApp()
	testCmd := &cli.Command{
		Name: "test-cmd",
		Action: func(ctx *cli.Context) error {
			_, _ = fmt.Fprint(app.Writer, makePathOutput(setting.AppWorkPath, setting.CustomPath, setting.CustomConf))
			return nil
		},
	}
	prepareSubcommandWithConfig(testCmd, appGlobalFlags())
	app.Commands = append(app.Commands, testCmd)
	app.DefaultCommand = testCmd.Name
	return app
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

	app := newTestApp()
	var envBackup []string
	for _, s := range os.Environ() {
		if strings.HasPrefix(s, "GITEA_") && strings.Contains(s, "=") {
			envBackup = append(envBackup, s)
		}
	}
	clearGiteaEnv := func() {
		for _, s := range os.Environ() {
			if strings.HasPrefix(s, "GITEA_") {
				_ = os.Unsetenv(s)
			}
		}
	}
	defer func() {
		clearGiteaEnv()
		for _, s := range envBackup {
			k, v, _ := strings.Cut(s, "=")
			_ = os.Setenv(k, v)
		}
	}()

	for _, c := range cases {
		clearGiteaEnv()
		for k, v := range c.env {
			_ = os.Setenv(k, v)
		}
		args := strings.Split(c.cmd, " ") // for test only, "split" is good enough
		out := new(strings.Builder)
		app.Writer = out
		err := app.Run(args)
		assert.NoError(t, err, c.cmd)
		assert.NotEmpty(t, c.exp, c.cmd)
		outStr := out.String()
		assert.Contains(t, outStr, c.exp, c.cmd)
	}
}
