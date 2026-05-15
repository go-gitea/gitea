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
		Commands: []*cli.Command{newDoctorCheckCommand()},
	}
	err := app.Run(t.Context(), []string{"./gitea", "check", "--run", "test-check"})
	assert.NoError(t, err)
	err = app.Run(t.Context(), []string{"./gitea", "check", "--run", "no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
	err = app.Run(t.Context(), []string{"./gitea", "check", "--run", "test-check,no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
}

func TestFilterFixChecksForAll(t *testing.T) {
	checks := []*doctor.Check{
		{Name: "safe"},
		{Name: "dangerous", IsDestructive: true},
	}

	filtered, skipped := filterFixChecksForAll(checks)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "safe", filtered[0].Name)
	assert.Len(t, skipped, 1)
	assert.Equal(t, "dangerous", skipped[0].Name)
}

func TestFilterChecksForAll(t *testing.T) {
	checks := []*doctor.Check{
		{Name: "storages"},
		{Name: "storage-attachments"},
		{Name: "storage-lfs"},
		{Name: "safe"},
	}

	filtered := filterChecksForAll(checks)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "storages", filtered[0].Name)
	assert.Equal(t, "safe", filtered[1].Name)

	filtered = filterChecksForAll([]*doctor.Check{{Name: "storage-attachments"}, {Name: "safe"}})
	assert.Len(t, filtered, 2)
	assert.Equal(t, "storage-attachments", filtered[0].Name)
	assert.Equal(t, "safe", filtered[1].Name)
}

func TestDoctorCheckFixMode(t *testing.T) {
	assert.Equal(t, "safe", doctorCheckFixMode(&doctor.Check{}))
	assert.Equal(t, "explicit", doctorCheckFixMode(&doctor.Check{IsDestructive: true}))
}
