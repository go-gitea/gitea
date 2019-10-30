// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"

	"gitea.com/macaron/cors"
)

var (
	// CORSConfig defines CORS settings
	CORSConfig cors.Options
	// EnableCORS defines whether CORS settings is enabled or not
	EnableCORS bool
)

func newCORSService() {
	sec := Cfg.Section("cors")
	// Check cors setting.
	EnableCORS = sec.Key("ENABLED").MustBool(false)

	maxAge := sec.Key("MAX_AGE").MustDuration(10 * time.Minute)

	CORSConfig = cors.Options{
		Scheme:           sec.Key("SCHEME").String(),
		AllowDomain:      sec.Key("ALLOW_DOMAIN").Strings(","),
		AllowSubdomain:   sec.Key("ALLOW_SUBDOMAIN").MustBool(),
		Methods:          sec.Key("METHODS").Strings(","),
		MaxAgeSeconds:    int(maxAge.Seconds()),
		AllowCredentials: sec.Key("ALLOW_CREDENTIALS").MustBool(),
	}

	if EnableCORS {
		log.Info("CORS Service Enabled")
	}
}
