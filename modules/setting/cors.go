// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
	"github.com/rs/cors"
)

// Cors handles CORS requests and allows other middlewares
// to check whetcher request marches CORS allowed origins.
var Cors *cors.Cors

// CORSConfig defines CORS settings
var CORSConfig = struct {
	Enabled          bool
	Scheme           string
	AllowDomain      []string
	AllowSubdomain   bool
	Methods          []string
	MaxAge           time.Duration
	AllowCredentials bool
	XFrameOptions    string
}{
	Enabled:       false,
	MaxAge:        10 * time.Minute,
	XFrameOptions: "SAMEORIGIN",
}

func newCORSService() {
	sec := Cfg.Section("cors")
	if err := sec.MapTo(&CORSConfig); err != nil {
		log.Fatal("Failed to map cors settings: %v", err)
	}

	if CORSConfig.Enabled {
		Cors = cors.New(cors.Options{
			AllowedOrigins:   CORSConfig.AllowDomain,
			AllowedMethods:   CORSConfig.Methods,
			AllowCredentials: CORSConfig.AllowCredentials,
			MaxAge:           int(CORSConfig.MaxAge.Seconds()),
		})

		log.Info("CORS Service Enabled")
	}
}
