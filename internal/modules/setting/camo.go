// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/internal/modules/log"

var Camo = struct {
	Enabled   bool
	ServerURL string `ini:"SERVER_URL"`
	HMACKey   string `ini:"HMAC_KEY"`
	Allways   bool
}{}

func loadCamoFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "camo", &Camo)
	if Camo.Enabled {
		if Camo.ServerURL == "" || Camo.HMACKey == "" {
			log.Fatal(`Camo settings require "SERVER_URL" and HMAC_KEY`)
		}
	}
}
