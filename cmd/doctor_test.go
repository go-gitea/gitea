// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/doctor"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestDoctorRun(t *testing.T) {
	doctor.Register(&doctor.Check{
		Title: "Test Check",
		Name:  "test-check",
		Run:   func(ctx context.Context, logger log.Logger, autofix bool) error { return nil },

		SkipDatabaseInitialization: true,
	})
	app := cli.NewApp()
	app.Commands = []*cli.Command{cmdDoctorCheck}
	err := app.Run([]string{"./gitea", "check", "--run", "test-check"})
	assert.NoError(t, err)
	err = app.Run([]string{"./gitea", "check", "--run", "no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
	err = app.Run([]string{"./gitea", "check", "--run", "test-check,no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
}
