// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"

	"github.com/go-chi/cors"
)

// CorsHandler return a http handler who set CORS options if enabled by config
func CorsHandler() func(next http.Handler) http.Handler {
	if setting.CORSConfig.Enabled {
		return cors.Handler(cors.Options{
			//Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			//setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		})
	} else {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
}
