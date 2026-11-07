// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Metrics settings
var Metrics = struct {
	Enabled                  bool
	Token                    string
	EnabledIssueByLabel      bool
	EnabledIssueByRepository bool
	EnabledRefCount          bool
	EnabledObjectCount       bool
}{
	Enabled:                  false,
	Token:                    "",
	EnabledIssueByLabel:      false,
	EnabledIssueByRepository: false,
	EnabledRefCount:          true,
	EnabledObjectCount:       true,
}

func loadMetricsFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "metrics", &Metrics)
}
