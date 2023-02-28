// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/setting/base"

func loadTaskFrom(rootCfg base.ConfigProvider) {
	sec := rootCfg.Section("queue.task")
	sec.Key("TYPE").MustString("channel")
	sec.Key("CONN_STR").MustString("redis://127.0.0.1:6379/0")
	sec.Key("LENGTH").MustInt(1000)
}
