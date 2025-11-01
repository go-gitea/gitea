// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"

	"code.gitea.io/gitea/modules/log"
)

// Webhook settings
var Webhook = struct {
	QueueLength     int
	DeliverTimeout  int
	SkipTLSVerify   bool
	AllowedHostList string
	Types           []string
	PagingNum       int
	ProxyURL        string
	ProxyURLFixed   *url.URL
	ProxyHosts      []string
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
	Webhook.AllowedHostList = sec.Key("ALLOWED_HOST_LIST").MustString("")
	Webhook.Types = []string{"gitea", "gogs", "slack", "discord", "dingtalk", "telegram", "msteams", "feishu", "matrix", "wechatwork", "packagist", "bark"}
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
