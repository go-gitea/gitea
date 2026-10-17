// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"cmp"
	"net/http"
	"path/filepath"
	"strings"

	"gitea.dev/modules/util"
)

// SessionConfig defines Session settings
var SessionConfig = struct {
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
	// SameSite declares if your cookie should be restricted to a first-party or same-site context. Valid strings are "none", "lax", "strict". Default is "lax"
	SameSite http.SameSite
}{
	Provider:    "memory", // the "Install" page doesn't load the [session] config
	CookieName:  "gitea_session",
	Gclifetime:  86400,
	Maxlifetime: 86400,
	SameSite:    http.SameSiteLaxMode,
}

func loadSessionFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("session")
	SessionConfig.Provider = sec.Key("PROVIDER").MustString("file")

	switch SessionConfig.Provider {
	case "redis":
		SessionConfig.ProviderConfig = sec.Key("PROVIDER_CONFIG").MustString(Redis.ConnStr)
	case "file":
		SessionConfig.ProviderConfig = sec.Key("PROVIDER_CONFIG").MustString(filepath.Join(AppDataPath, "sessions"))
		if !filepath.IsAbs(SessionConfig.ProviderConfig) {
			// Although the "data path" should be used as Gitea's "data" base directory (work path sometimes is not writable),
			// document says the relative session path is based on the "work path", so keep the behavior
			SessionConfig.ProviderConfig = filepath.Join(AppWorkPath, SessionConfig.ProviderConfig)
		}
		checkOverlappedPath("[session].PROVIDER_CONFIG", SessionConfig.ProviderConfig)
	default:
		SessionConfig.ProviderConfig = sec.Key("PROVIDER_CONFIG").String()
	}

	SessionConfig.CookieName = sec.Key("COOKIE_NAME").MustString("gitea_session")
	// HINT: INSTALL-PAGE-COOKIE-INIT: the cookie system is not properly initialized on the "Install" page, so there is no CookiePath
	SessionConfig.CookiePath = util.IfZero(AppSubURL, "/")
	SessionConfig.Secure = sec.Key("COOKIE_SECURE").MustBool(strings.HasPrefix(strings.ToLower(AppURL), "https://"))
	SessionConfig.Gclifetime = cmp.Or(max(sec.Key("GC_INTERVAL_TIME").MustInt64(86400), 0), 3600)
	SessionConfig.Maxlifetime = cmp.Or(max(sec.Key("SESSION_LIFE_TIME").MustInt64(86400), 0), SessionConfig.Gclifetime)
	SessionConfig.Domain = sec.Key("DOMAIN").String()
	samesiteString := sec.Key("SAME_SITE").In("lax", []string{"none", "lax", "strict"})
	switch strings.ToLower(samesiteString) {
	case "none":
		SessionConfig.SameSite = http.SameSiteNoneMode
	case "strict":
		SessionConfig.SameSite = http.SameSiteStrictMode
	default:
		SessionConfig.SameSite = http.SameSiteLaxMode
	}
}
