// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Actions settings
var (
	Actions = struct {
		LogStorage        Storage // how the created logs should be stored
		ArtifactStorage   Storage // how the created artifacts should be stored
		Enabled           bool
		DefaultActionsURL string `ini:"DEFAULT_ACTIONS_URL"`
	}{
		Enabled:           false,
		DefaultActionsURL: "https://gitea.com",
	}
)

func loadActionsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("actions")
	if err := sec.MapTo(&Actions); err != nil {
		log.Fatal("Failed to map Actions settings: %v", err)
	}

	actionsSec := rootCfg.Section("actions.artifacts")
	storageType := actionsSec.Key("STORAGE_TYPE").MustString("")

	Actions.LogStorage = getStorage(rootCfg, "actions_log", "", nil)
	Actions.ArtifactStorage = getStorage(rootCfg, "actions_artifacts", storageType, actionsSec)
}
