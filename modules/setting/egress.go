// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net"

	"gitea.dev/modules/log"
)

var Egress = struct {
	GitProxyListenAddr string
}{
	GitProxyListenAddr: "127.0.0.1:0",
}

func loadEgressFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("egress")
	Egress.GitProxyListenAddr = sec.Key("GIT_PROXY_LISTEN_ADDR").MustString("127.0.0.1:0")
	if _, _, err := net.SplitHostPort(Egress.GitProxyListenAddr); err != nil {
		log.Fatal("Invalid [egress] GIT_PROXY_LISTEN_ADDR %q: %v", Egress.GitProxyListenAddr, err)
	}
}
