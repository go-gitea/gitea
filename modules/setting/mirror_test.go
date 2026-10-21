// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting_test

import (
	"fmt"
	"strconv"
	"testing"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/require"
)

func TestMirrorBackupLimitDefaultsToDisabled(t *testing.T) {
	require.Zero(t, setting.Mirror.DefaultForcePushBackupLimit)
}

func TestMirrorBackupLimitLoadsValidValues(t *testing.T) {
	for _, limit := range []int{-1, 0, 1, 12} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			defer test.MockVariableValue(&setting.Mirror)()
			cfg, err := setting.NewConfigProviderFromData(fmt.Sprintf("[mirror]\nDEFAULT_FORCE_PUSH_BACKUP_LIMIT = %d\n", limit))
			require.NoError(t, err)
			setting.LoadMirrorFrom(cfg)
			require.Equal(t, limit, setting.Mirror.DefaultForcePushBackupLimit)
		})
	}
}

func TestMirrorBackupLimitRejectsValuesBelowUnlimited(t *testing.T) {
	defer test.MockVariableValue(&setting.Mirror)()
	defer test.MockVariableValue(&log.OsExiter, func(code int) { panic(code) })()
	cfg, err := setting.NewConfigProviderFromData("[mirror]\nDEFAULT_FORCE_PUSH_BACKUP_LIMIT = -2\n")
	require.NoError(t, err)
	require.PanicsWithValue(t, 1, func() { setting.LoadMirrorFrom(cfg) })
}
