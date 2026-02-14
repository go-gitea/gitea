// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strconv"

	"code.gitea.io/gitea/modules/log"
)

var Camo = struct {
	Enabled   bool
	ServerURL string `ini:"SERVER_URL"`
	HMACKey   string `ini:"HMAC_KEY"`
	Always    bool
}{}

func loadCamoFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "camo", &Camo)
	if Camo.Enabled {
		oldValue := rootCfg.Section("camo").Key("ALLWAYS").MustString("")
		if oldValue != "" {
			log.Warn("camo.ALLWAYS is deprecated, use camo.ALWAYS instead")
			Camo.Always, _ = strconv.ParseBool(oldValue)
		}

		if Camo.ServerURL == "" || Camo.HMACKey == "" {
			log.Fatal(`Camo settings require "SERVER_URL" and HMAC_KEY`)
		}
	}
}
