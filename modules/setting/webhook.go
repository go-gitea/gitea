// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"

	"gitea.dev/modules/log"
)

// Webhook settings
var Webhook = struct {
	QueueLength    int
	DeliverTimeout int
	SkipTLSVerify  bool
	Types          []string
	PagingNum      int
	ProxyURL       string
	ProxyURLFixed  *url.URL
	ProxyHosts     []string
}{
	QueueLength:    1000,
	DeliverTimeout: 5,
	SkipTLSVerify:  false,
	PagingNum:      10,
	ProxyURL:       "",
	ProxyHosts:     []string{},
}

func loadWebhookFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("webhook")
	Webhook.QueueLength = sec.Key("QUEUE_LENGTH").MustInt(1000)
	Webhook.DeliverTimeout = sec.Key("DELIVER_TIMEOUT").MustInt(5)
	Webhook.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool()
	allowedWebhookHosts := sec.Key("ALLOWED_HOST_LIST").MustString("")
	// Bridge the deprecated [webhook] ALLOWED_HOST_LIST to the global [security] ALLOWED_HOST_LIST for one
	// release, but only when the operator has not explicitly configured the new key. Comparing against the
	// "external" default cannot tell an explicit [security] ALLOWED_HOST_LIST = external apart from the
	// default and would silently overwrite it, so use the presence captured before the default was
	// materialized. This workaround is to be removed in v28.0.0.
	if allowedWebhookHosts != "" && !securityAllowedHostListExplicit {
		Security.AllowedHostList = allowedWebhookHosts
		deprecatedSetting(rootCfg, "webhook", "ALLOWED_HOST_LIST", "security", "ALLOWED_HOST_LIST", "v28.0.0")
	}
	Webhook.Types = []string{"gitea", "gogs", "slack", "discord", "dingtalk", "telegram", "msteams", "feishu", "matrix", "wechatwork", "packagist"}
	Webhook.PagingNum = sec.Key("PAGING_NUM").MustInt(10)
	Webhook.ProxyURL = sec.Key("PROXY_URL").MustString("")
	if Webhook.ProxyURL != "" {
		var err error
		Webhook.ProxyURLFixed, err = url.Parse(Webhook.ProxyURL)
		if err != nil {
			log.Error("Webhook PROXY_URL is not valid")
			Webhook.ProxyURL = ""
		}
	}
	Webhook.ProxyHosts = sec.Key("PROXY_HOSTS").Strings(",")
}
