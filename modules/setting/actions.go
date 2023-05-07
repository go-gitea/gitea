// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
)

// Actions settings
var (
	Actions = struct {
		*Storage          // how the created logs should be stored
		Enabled           bool
		DefaultActionsURL string `ini:"DEFAULT_ACTIONS_URL"`
	}{
		Enabled:           false,
		DefaultActionsURL: "https://gitea.com",
	}
)

func loadActionsFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("actions")
	if err := sec.MapTo(&Actions); err != nil {
		return fmt.Errorf("failed to map Actions settings: %v", err)
	}

	// don't support to read configuration from [actions]
	var err error
	Actions.Storage, err = getStorage(rootCfg, "actions_log", nil, "")
	return err
}
