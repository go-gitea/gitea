// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Actions settings
var (
	Actions = struct {
		Storage           // how the created logs should be stored
		Enabled           bool
		DefaultActionsURL string `ini:"DEFAULT_ACTIONS_URL"`
	}{
		Enabled:           false,
		DefaultActionsURL: "https://gitea.com",
	}
)

func newActions() {
	sec := Cfg.Section("actions")
	if err := sec.MapTo(&Actions); err != nil {
		log.Fatal("Failed to map Actions settings: %v", err)
	}

	Actions.Storage = getStorage("actions_log", "", nil)
}
