// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/setting/base"

// Metrics settings
var Metrics = struct {
	Enabled                  bool
	Token                    string
	EnabledIssueByLabel      bool
	EnabledIssueByRepository bool
}{
	Enabled:                  false,
	Token:                    "",
	EnabledIssueByLabel:      false,
	EnabledIssueByRepository: false,
}

func loadMetricsFrom(rootCfg base.ConfigProvider) {
	base.MustMapSetting(rootCfg, "metrics", &Metrics)
}
