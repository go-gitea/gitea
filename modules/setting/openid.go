// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/hostmatcher"
)

// OpenID settings
var OpenID = struct {
	EnableSignIn bool
	EnableSignUp bool
	Allowlist    *hostmatcher.HostMatchList
	Blocklist    *hostmatcher.HostMatchList
}{
	EnableSignIn: false,
	EnableSignUp: false,
	Allowlist:    nil,
	Blocklist:    nil,
}

var defaultOpenIDBlocklist = []string{
	"localhost",
	hostmatcher.MatchBuiltinLoopback,
	hostmatcher.MatchBuiltinPrivate,
	"fe80::/10",
}

func splitOpenIDHostList(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
}

// loadOpenIDSetting loads OpenID settings from rootCfg, depends on service settings,
// so should be called after loadServiceSettings.
func loadOpenIDSetting(rootCfg ConfigProvider) {
	sec := rootCfg.Section("openid")
	OpenID.EnableSignIn = sec.Key("ENABLE_OPENID_SIGNIN").MustBool(false)
	OpenID.EnableSignUp = sec.Key("ENABLE_OPENID_SIGNUP").MustBool(!Service.DisableRegistration && OpenID.EnableSignIn)
	OpenID.Allowlist = nil
	OpenID.Blocklist = nil
	allowlist := splitOpenIDHostList(sec.Key("WHITELISTED_URIS").String())
	if len(allowlist) != 0 {
		OpenID.Allowlist = hostmatcher.ParseHostMatchList("openid.WHITELISTED_URIS", strings.Join(allowlist, ","))
	}
	blocklist := splitOpenIDHostList(sec.Key("BLACKLISTED_URIS").String())
	if len(blocklist) == 0 {
		blocklist = defaultOpenIDBlocklist
	}
	if len(blocklist) != 0 {
		OpenID.Blocklist = hostmatcher.ParseHostMatchList("openid.BLACKLISTED_URIS", strings.Join(blocklist, ","))
	}
}
