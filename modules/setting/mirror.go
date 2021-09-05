// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// Mirror settings
	Mirror = struct {
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
)

func newMirror() {
	if err := Cfg.Section("mirror").MapTo(&Mirror); err != nil {
		log.Fatal("Failed to map Mirror settings: %v", err)
	}

	// fallback to old config repository.DISABLE_MIRRORS
	if Cfg.Section("repository").Key("DISABLE_MIRRORS").MustBool(false) {
		log.Warn("Deprecated DISABLE_MIRRORS config is used, please change your config and use the optins within [mirror] section")
		// TODO: enable on v1.17.0: log.Error("Deprecated fallback used, will be removed in v1.18.0")
		Mirror.DisableNewPull = true
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
		log.Warn("Mirror.DefaultInterval is less than Mirror.MinInterval, set to 8 hours")
		Mirror.DefaultInterval = time.Hour * 8
	}
}
