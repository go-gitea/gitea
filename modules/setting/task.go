// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// FIXME: DEPRECATED to be removed in v1.18.0
// - will need to set default for [queue.task] LENGTH to 1000 though
func loadTaskFrom(rootCfg ConfigProvider) {
	taskSec := rootCfg.Section("task")
	queueTaskSec := rootCfg.Section("queue.task")

	deprecatedSetting(rootCfg, "task", "QUEUE_TYPE", "queue.task", "TYPE")
	deprecatedSetting(rootCfg, "task", "QUEUE_CONN_STR", "queue.task", "CONN_STR")
	deprecatedSetting(rootCfg, "task", "QUEUE_LENGTH", "queue.task", "LENGTH")

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
