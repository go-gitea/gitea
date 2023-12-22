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
	AllowDomain      []string // this option is from legacy code, it should be called "AllowedOrigins" actually
	Methods          []string
	MaxAge           time.Duration
	AllowCredentials bool
	Headers          []string
	XFrameOptions    string
}{
	AllowDomain:   []string{"*"},
	Methods:       []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	Headers:       []string{"Content-Type", "User-Agent"},
	MaxAge:        10 * time.Minute,
	XFrameOptions: "SAMEORIGIN",
}

func loadCorsFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "cors", &CORSConfig)
	if CORSConfig.Enabled {
		log.Info("CORS Service Enabled")
	}
}
