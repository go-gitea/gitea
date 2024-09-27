// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var ReverseProxyAuth = struct {
	Enabled                        bool
	EnableReverseProxyAuthAPI      bool
	EnableReverseProxyAutoRegister bool
	EnableReverseProxyEmail        bool
	EnableReverseProxyFullName     bool
	ReverseProxyAuthUser           string
	ReverseProxyAuthEmail          string
	ReverseProxyAuthFullName       string
	ReverseProxyLimit              int
	ReverseProxyTrustedProxies     []string
}{}

func loadReverseProxyAuthFrom(rootCfg ConfigProvider) {
	serviceSec := rootCfg.Section("service")

	ReverseProxyAuth.Enabled = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	ReverseProxyAuth.EnableReverseProxyAuthAPI = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION_API").MustBool()
	ReverseProxyAuth.EnableReverseProxyAutoRegister = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	ReverseProxyAuth.EnableReverseProxyEmail = serviceSec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	ReverseProxyAuth.EnableReverseProxyFullName = serviceSec.Key("ENABLE_REVERSE_PROXY_FULL_NAME").MustBool()

	securitySec := rootCfg.Section("security")
	ReverseProxyAuth.ReverseProxyAuthUser = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	ReverseProxyAuth.ReverseProxyAuthEmail = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_EMAIL").MustString("X-WEBAUTH-EMAIL")
	ReverseProxyAuth.ReverseProxyAuthFullName = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_FULL_NAME").MustString("X-WEBAUTH-FULLNAME")

	ReverseProxyAuth.ReverseProxyLimit = securitySec.Key("REVERSE_PROXY_LIMIT").MustInt(1)
	ReverseProxyAuth.ReverseProxyTrustedProxies = securitySec.Key("REVERSE_PROXY_TRUSTED_PROXIES").Strings(",")
	if len(ReverseProxyAuth.ReverseProxyTrustedProxies) == 0 {
		ReverseProxyAuth.ReverseProxyTrustedProxies = []string{"127.0.0.0/8", "::1/128"}
	}
}
