// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RegisterTask requires an "admin.dashboard.<name>" translation, so a task
// without a locale entry fails server startup.
func TestRegisterReEmitUnresolvedActionRuns(t *testing.T) {
	setting.SetupGiteaTestEnv()
	defer test.MockVariableValue(&setting.Langs, []string{"en-US"})()
	defer test.MockVariableValue(&setting.Names, []string{"English"})()
	translation.InitLocales(t.Context())

	cfg, err := setting.NewConfigProviderFromData(`
[cron.reemit_unresolved_action_runs]
ENABLED = true
RUN_AT_START = true
SCHEDULE = "@every 5m"
`)
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()

	registerReEmitUnresolvedActionRuns()
	defer scheduler.RemoveByTags("reemit_unresolved_action_runs")

	task := GetTask("reemit_unresolved_action_runs")
	require.NotNil(t, task)
	assert.Equal(t, "@every 5m", task.config.GetSchedule())
}
