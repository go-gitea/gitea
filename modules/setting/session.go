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
	// This ensures that VirtualSessionProvider is available
	_ "code.gitea.io/gitea/modules/session"

	"github.com/go-macaron/session"
)

var (
	// SessionConfig difines Session settings
	SessionConfig session.Options
)

func newSessionService() {
	SessionConfig.Provider = Cfg.Section("session").Key("PROVIDER").In("memory",
		[]string{"memory", "file", "redis", "mysql", "postgres", "couchbase", "memcache", "nodb"})
	SessionConfig.ProviderConfig = strings.Trim(Cfg.Section("session").Key("PROVIDER_CONFIG").MustString(path.Join(AppDataPath, "sessions")), "\" ")
	if SessionConfig.Provider == "file" && !filepath.IsAbs(SessionConfig.ProviderConfig) {
		SessionConfig.ProviderConfig = path.Join(AppWorkPath, SessionConfig.ProviderConfig)
	}
	SessionConfig.CookieName = Cfg.Section("session").Key("COOKIE_NAME").MustString("i_like_gitea")
	SessionConfig.CookiePath = AppSubURL
	SessionConfig.Secure = Cfg.Section("session").Key("COOKIE_SECURE").MustBool(false)
	SessionConfig.Gclifetime = Cfg.Section("session").Key("GC_INTERVAL_TIME").MustInt64(86400)
	SessionConfig.Maxlifetime = Cfg.Section("session").Key("SESSION_LIFE_TIME").MustInt64(86400)

	shadowConfig, err := json.Marshal(SessionConfig)
	if err != nil {
		log.Fatal("Can't shadow session config: %v", err)
	}
	SessionConfig.ProviderConfig = string(shadowConfig)
	SessionConfig.Provider = "VirtualSession"

	log.Info("Session Service Enabled")
}
