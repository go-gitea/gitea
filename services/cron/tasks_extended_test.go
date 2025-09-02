// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_GCLFSConfig(t *testing.T) {
	cfg, err := setting.NewConfigProviderFromData(`
[cron.gc_lfs]
ENABLED = true
RUN_AT_START = true
SCHEDULE = "@every 2h"
OLDER_THAN = "1h"
LAST_UPDATED_MORE_THAN_AGO = "7h"
NUMBER_TO_CHECK_PER_REPO = 10
PROPORTION_TO_CHECK_PER_REPO = 0.1
`)
	assert.NoError(t, err)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()

	config := &GCLFSConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 24h",
		},
		OlderThan:                24 * time.Hour * 7,
		LastUpdatedMoreThanAgo:   24 * time.Hour * 3,
		NumberToCheckPerRepo:     100,
		ProportionToCheckPerRepo: 0.6,
	}

	_, err = setting.GetCronSettings("gc_lfs", config)
	assert.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.True(t, config.RunAtStart)
	assert.Equal(t, "@every 2h", config.Schedule)
	assert.Equal(t, 1*time.Hour, config.OlderThan)
	assert.Equal(t, 7*time.Hour, config.LastUpdatedMoreThanAgo)
	assert.Equal(t, int64(10), config.NumberToCheckPerRepo)
	assert.InDelta(t, 0.1, config.ProportionToCheckPerRepo, 0.001)
}
