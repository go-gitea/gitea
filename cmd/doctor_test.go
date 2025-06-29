// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/doctor"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestDoctorRun(t *testing.T) {
	doctor.Register(&doctor.Check{
		Title: "Test Check",
		Name:  "test-check",
		Run:   func(ctx context.Context, logger log.Logger, autofix bool) error { return nil },

		SkipDatabaseInitialization: true,
	})
	app := &cli.Command{
		Commands: []*cli.Command{cmdDoctorCheck},
	}
	err := app.Run(t.Context(), []string{"./gitea", "check", "--run", "test-check"})
	assert.NoError(t, err)
	err = app.Run(t.Context(), []string{"./gitea", "check", "--run", "no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
	err = app.Run(t.Context(), []string{"./gitea", "check", "--run", "test-check,no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
}
