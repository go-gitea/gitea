// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
)

// CORSConfig defines CORS settings
var CORSConfig = struct {
	Enabled          bool
	Scheme           string
	AllowDomain      []string
	AllowSubdomain   bool
	Methods          []string
	MaxAge           time.Duration
	AllowCredentials bool
	Headers          []string
	XFrameOptions    string
}{
	Enabled:       false,
	MaxAge:        10 * time.Minute,
	Headers:       []string{"Content-Type", "User-Agent"},
	XFrameOptions: "SAMEORIGIN",
}

func loadCorsFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "cors", &CORSConfig)
	if CORSConfig.Enabled {
		log.Info("CORS Service Enabled")
	}
}
