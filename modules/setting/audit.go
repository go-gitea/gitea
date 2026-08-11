// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var Audit = struct {
	Enabled bool
}{
	Enabled: false,
}

func loadAuditFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "audit", &Audit)
}
