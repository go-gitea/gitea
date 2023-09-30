package proxy

import (
	"code.gitea.io/gitea/modules/setting"
	"golang.org/x/net/proxy"
)

func SMTPProxy() (proxy.Dialer, error) {
	cfg := setting.SMTPProxy
	if !cfg.Enabled {
		return proxy.Direct, nil
	}

	var (
		dialer proxy.Dialer
		err    error
	)
	if cfg.ProxyURLFixed != nil {
		dialer, err = proxy.FromURL(cfg.ProxyURLFixed, proxy.Direct)
		if err != nil {
			return nil, err
		}

		if cfg.NoProxy != "" {
			p := proxy.NewPerHost(dialer, proxy.Direct)
			p.AddFromString(cfg.NoProxy)
			return p, nil
		}
	}

	dialer = proxy.FromEnvironment()

	return dialer, nil
}
