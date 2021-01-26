// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"encoding/json"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

var (
	// SessionConfig difines Session settings
	SessionConfig = struct {
		Provider string
		// Provider configuration, it's corresponding to provider.
		ProviderConfig string
		// Cookie name to save session ID. Default is "MacaronSession".
		CookieName string
		// Cookie path to store. Default is "/".
		CookiePath string
		// GC interval time in seconds. Default is 3600.
		Gclifetime int64
		// Max life time in seconds. Default is whatever GC interval time is.
		Maxlifetime int64
		// Use HTTPS only. Default is false.
		Secure bool
		// Cookie domain name. Default is empty.
		Domain string
	}{
		CookieName:  "i_like_gitea",
		Gclifetime:  86400,
		Maxlifetime: 86400,
	}
)

func newSessionService() {
	sec := Cfg.Section("session")
	SessionConfig.Provider = sec.Key("PROVIDER").In("memory",
		[]string{"memory", "file", "redis", "mysql", "postgres", "couchbase", "memcache"})
	SessionConfig.ProviderConfig = strings.Trim(sec.Key("PROVIDER_CONFIG").MustString(path.Join(AppDataPath, "sessions")), "\" ")
	if SessionConfig.Provider == "file" && !filepath.IsAbs(SessionConfig.ProviderConfig) {
		SessionConfig.ProviderConfig = path.Join(AppWorkPath, SessionConfig.ProviderConfig)
	}
	SessionConfig.CookieName = sec.Key("COOKIE_NAME").MustString("i_like_gitea")
	SessionConfig.CookiePath = AppSubURL
	SessionConfig.Secure = sec.Key("COOKIE_SECURE").MustBool(false)
	SessionConfig.Gclifetime = sec.Key("GC_INTERVAL_TIME").MustInt64(86400)
	SessionConfig.Maxlifetime = sec.Key("SESSION_LIFE_TIME").MustInt64(86400)
	SessionConfig.Domain = sec.Key("DOMAIN").String()

	shadowConfig, err := json.Marshal(SessionConfig)
	if err != nil {
		log.Fatal("Can't shadow session config: %v", err)
	}
	SessionConfig.ProviderConfig = string(shadowConfig)
	SessionConfig.Provider = "VirtualSession"

	log.Info("Session Service Enabled")
}
