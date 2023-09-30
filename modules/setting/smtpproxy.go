package setting

import (
	"code.gitea.io/gitea/modules/log"
	"net/url"
)

var SMTPProxy = struct {
	Enabled  bool   `ini:"ENABLED"`
	ProxyURL string `ini:"PROXY_URL"`
	NoProxy  string `ini:"NO_PROXY"`

	ProxyURLFixed *url.URL `-`
}{
	Enabled:  false,
	ProxyURL: "",
}

func loadSMTPProxyFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("smtp_proxy")
	SMTPProxy.Enabled = sec.Key("ENABLED").MustBool(false)
	SMTPProxy.ProxyURL = sec.Key("PROXY_URL").MustString("")
	SMTPProxy.NoProxy = sec.Key("NO_PROXY").MustString("")
	if SMTPProxy.ProxyURL != "" {
		var err error
		SMTPProxy.ProxyURLFixed, err = url.Parse(SMTPProxy.ProxyURL)
		if err != nil {
			log.Error("Global PROXY_URL is not valid")
			SMTPProxy.ProxyURL = ""
		}

		if SMTPProxy.ProxyURLFixed.Scheme != "socks5" {
			log.Error("The scheme of SMTP PROXY_URL should be socks5")
			SMTPProxy.ProxyURL = ""
			SMTPProxy.ProxyURLFixed = nil
		}
	}
}
