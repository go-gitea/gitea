// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

var (
	// Task settings
	Task = struct {
		QueueType    string
		QueueLength  int
		QueueConnStr string
	}{
		QueueType:    ChannelQueueType,
		QueueLength:  1000,
		QueueConnStr: "addrs=127.0.0.1:6379 db=0",
	}
)

func newTaskService() {
	sec := Cfg.Section("task")
	Task.QueueType = sec.Key("QUEUE_TYPE").MustString(ChannelQueueType)
	Task.QueueLength = sec.Key("QUEUE_LENGTH").MustInt(1000)
	Task.QueueConnStr = sec.Key("QUEUE_CONN_STR").MustString("addrs=127.0.0.1:6379 db=0")
}
