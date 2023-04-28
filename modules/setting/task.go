// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// DEPRECATED should not be removed because users maybe upgrade from lower version to the latest version
// if these are removed, the warning will not be shown
// - will need to set default for [queue.task] LENGTH to 1000 though
func loadTaskFrom(rootCfg ConfigProvider) {
	taskSec := rootCfg.Section("task")
	queueTaskSec := rootCfg.Section("queue.task")

	deprecatedSetting(rootCfg, "task", "QUEUE_TYPE", "queue.task", "TYPE", "v1.19.0")
	deprecatedSetting(rootCfg, "task", "QUEUE_CONN_STR", "queue.task", "CONN_STR", "v1.19.0")
	deprecatedSetting(rootCfg, "task", "QUEUE_LENGTH", "queue.task", "LENGTH", "v1.19.0")

	switch taskSec.Key("QUEUE_TYPE").MustString("channel") {
	case "channel":
		queueTaskSec.Key("TYPE").MustString("persistable-channel")
		queueTaskSec.Key("CONN_STR").MustString(taskSec.Key("QUEUE_CONN_STR").MustString(""))
	case "redis":
		queueTaskSec.Key("TYPE").MustString("redis")
		queueTaskSec.Key("CONN_STR").MustString(taskSec.Key("QUEUE_CONN_STR").MustString("addrs=127.0.0.1:6379 db=0"))
	}
	queueTaskSec.Key("LENGTH").MustInt(taskSec.Key("QUEUE_LENGTH").MustInt(1000))
}
