// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var ReverseProxy = struct {
	EnableAuth         bool
	EnableAuthAPI      bool
	EnableAutoRegister bool
	EnableEmail        bool
	EnableFullName     bool

	AuthUser       string
	AuthEmail      string
	AuthFullName   string
	Limit          int
	TrustedProxies []string
}{}

func loadReverseProxyFrom(rootCfg ConfigProvider) {
	serviceSec := rootCfg.Section("service")
	ReverseProxy.EnableAuth = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	ReverseProxy.EnableAuthAPI = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION_API").MustBool()
	ReverseProxy.EnableAutoRegister = serviceSec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	ReverseProxy.EnableEmail = serviceSec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	ReverseProxy.EnableFullName = serviceSec.Key("ENABLE_REVERSE_PROXY_FULL_NAME").MustBool()

	securitySec := rootCfg.Section("security")
	ReverseProxy.AuthUser = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	ReverseProxy.AuthEmail = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_EMAIL").MustString("X-WEBAUTH-EMAIL")
	ReverseProxy.AuthFullName = securitySec.Key("REVERSE_PROXY_AUTHENTICATION_FULL_NAME").MustString("X-WEBAUTH-FULLNAME")
	ReverseProxy.Limit = securitySec.Key("REVERSE_PROXY_LIMIT").MustInt(1)
	ReverseProxy.TrustedProxies = securitySec.Key("REVERSE_PROXY_TRUSTED_PROXIES").Strings(",")
	if len(ReverseProxy.TrustedProxies) == 0 {
		ReverseProxy.TrustedProxies = []string{"127.0.0.0/8", "::1/128"}
	}
}
