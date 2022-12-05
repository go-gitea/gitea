// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Bots settings
var (
	Bots = struct {
		Storage
		Enabled        bool
		DefaultBotsURL string
	}{
		Enabled:        false,
		DefaultBotsURL: "https://gitea.com",
	}
)

func newBots() {
	sec := Cfg.Section("bots")
	if err := sec.MapTo(&Bots); err != nil {
		log.Fatal("Failed to map Bots settings: %v", err)
	}

	Bots.Storage = getStorage("bots_log", "", nil)
}
