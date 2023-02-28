// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting/base"
)

// Mirror settings
var Mirror = struct {
	Enabled         bool
	DisableNewPull  bool
	DisableNewPush  bool
	DefaultInterval time.Duration
	MinInterval     time.Duration
}{
	Enabled:         true,
	DisableNewPull:  false,
	DisableNewPush:  false,
	MinInterval:     10 * time.Minute,
	DefaultInterval: 8 * time.Hour,
}

func loadMirrorFrom(rootCfg base.ConfigProvider) {
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
		if time.Hour*8 < Mirror.MinInterval {
			Mirror.DefaultInterval = Mirror.MinInterval
		} else {
			Mirror.DefaultInterval = time.Hour * 8
		}
		log.Warn("Mirror.DefaultInterval is less than Mirror.MinInterval, set to %s", Mirror.DefaultInterval.String())
	}
}
