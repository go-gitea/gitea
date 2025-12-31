// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
)

// GitVM settings
var GitVM = struct {
	Enabled bool
	Dir     string
}{
	Enabled: true,
}

func loadGitVMFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("gitvm")
	GitVM.Enabled = sec.Key("ENABLED").MustBool(true)
	GitVM.Dir = sec.Key("DIR").MustString(filepath.Join(AppDataPath, "gitvm"))
}
