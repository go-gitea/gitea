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

// TestRegisterReEmitUnresolvedActionRuns guards the registration of the
// reemit_unresolved_action_runs cron task. RegisterTask requires a
// "admin.dashboard.<name>" translation for every task, so a task without a
// locale entry makes the server fail to start: RegisterTaskFatal calls
// log.Fatal during cron.Init.
// https://github.com/go-gitea/gitea/pull/40290
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
	defer func() {
		for _, j := range scheduler.Jobs() {
			for _, tag := range j.Tags() {
				if tag == "reemit_unresolved_action_runs" {
					_ = scheduler.RemoveJob(j.ID())
				}
			}
		}
	}()

	task := GetTask("reemit_unresolved_action_runs")
	require.NotNil(t, task)
	assert.Equal(t, "@every 5m", task.config.GetSchedule())
}
