// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-macaron/cors"
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
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	maxAge := sec.Key("MAX_AGE").MustDuration(10 * time.Minute)

	CORSConfig = cors.Options{
		Scheme:           sec.Key("SCHEME").String(),
		AllowDomain:      sec.Key("ALLOW_DOMAIN").String(),
		AllowSubdomain:   sec.Key("ALLOW_SUBDOMAIN").MustBool(),
		Methods:          sec.Key("METHODS").Strings(","),
		MaxAgeSeconds:    maxAge.Seconds(),
		AllowCredentials: sec.Key("ALLOW_CREDENTIALS").MustBool(),
	}
	EnableCORS = true

	log.Info("CORS Service Enabled")
}
