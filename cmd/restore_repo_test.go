// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func TestRestoreRepoFlags(t *testing.T) {
	app := cli.NewApp()
	called := false
	CmdRestoreRepository.Action = func(c *cli.Context) {
		assert.EqualValues(t, []string{"issues", "labels"}, c.StringSlice("units"))
		called = true
	}
	app.Commands = []cli.Command{
		CmdRestoreRepository,
	}
	err := app.Run([]string{"gitea", "restore-repo", "--units", "issues", "--units", "labels"})
	assert.True(t, called, "CmdRestoreRepository.Action")
	assert.NoError(t, err)
}

func TestRestoreRepoFlagsDefaults(t *testing.T) {
	app := cli.NewApp()
	called := false
	CmdRestoreRepository.Action = func(c *cli.Context) {
		assert.EqualValues(t, ([]string)(defaultUnits), c.StringSlice("units"))
		called = true
	}
	app.Commands = []cli.Command{
		CmdRestoreRepository,
	}
	err := app.Run([]string{"gitea", "restore-repo"})
	assert.True(t, called, "CmdRestoreRepository.Action")
	assert.NoError(t, err)
}
