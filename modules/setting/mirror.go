// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"gitea.dev/modules/log"
)

// Mirror settings
var Mirror = struct {
	Enabled                     bool
	DisableNewPull              bool
	DisableNewPush              bool
	DefaultInterval             time.Duration
	MinInterval                 time.Duration
	DefaultForcePushBackupLimit int
}{
	Enabled:                     true,
	DisableNewPull:              false,
	DisableNewPush:              false,
	MinInterval:                 10 * time.Minute,
	DefaultInterval:             8 * time.Hour,
	DefaultForcePushBackupLimit: 0,
}

func loadMirrorFrom(rootCfg ConfigProvider) {
	if err := rootCfg.Section("mirror").MapTo(&Mirror); err != nil {
		log.Fatal("Failed to map Mirror settings: %v", err)
	}

	if !Mirror.Enabled {
		Mirror.DisableNewPull = true
		Mirror.DisableNewPush = true
	}

	if Mirror.MinInterval.Minutes() < 1 {
		log.Warn("Mirror.MinInterval is too low, set to 1 minute")
		Mirror.MinInterval = 1 * time.Minute
	}
	if Mirror.DefaultInterval < Mirror.MinInterval {
		Mirror.DefaultInterval = max(time.Hour*8, Mirror.MinInterval)
		log.Warn("Mirror.DefaultInterval is less than Mirror.MinInterval, set to %s", Mirror.DefaultInterval.String())
	}
	if Mirror.DefaultForcePushBackupLimit < -1 {
		log.Fatal("Invalid [mirror].DEFAULT_FORCE_PUSH_BACKUP_LIMIT: %d (must be -1, 0, or positive)", Mirror.DefaultForcePushBackupLimit)
	}
}
