// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"

	"github.com/gobwas/glob"

	"code.gitea.io/gitea/modules/log"
)

// Proxy settings
var Proxy = struct {
	Enabled          bool     `ini:"PROXY_ENABLED"`
	ProxyURL         string   `ini:"PROXY_URL"`
	ProxyHosts       []string `ini:"PROXY_HOSTS"`
	SMTPProxyEnabled bool     `ini:"SMTP_PROXY_ENABLED"`

	ProxyURLFixed *url.URL    `-`
	HostMatchers  []glob.Glob `-`
}{
	Enabled:          false,
	SMTPProxyEnabled: false,
	ProxyURL:         "",
	ProxyHosts:       []string{},

	HostMatchers: []glob.Glob{},
}

func loadProxyFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("proxy")
	Proxy.Enabled = sec.Key("PROXY_ENABLED").MustBool(false)
	Proxy.SMTPProxyEnabled = sec.Key("SMTP_PROXY_ENABLED").MustBool(false)
	Proxy.ProxyURL = sec.Key("PROXY_URL").MustString("")
	Proxy.ProxyHosts = sec.Key("PROXY_HOSTS").Strings(",")

	ParseProxy()
}

func ParseProxy() error {
	if Proxy.ProxyURL != "" {
		var err error
		Proxy.ProxyURLFixed, err = url.Parse(Proxy.ProxyURL)
		if err != nil {
			log.Error("Global PROXY_URL is not valid")
			Proxy.ProxyURL = ""
		}
	}

	for _, h := range Proxy.ProxyHosts {
		if g, err := glob.Compile(h); err == nil {
			Proxy.HostMatchers = append(Proxy.HostMatchers, g)
		} else {
			log.Error("glob.Compile %s failed: %v", h, err)
			return err
		}
	}

	return nil
}
