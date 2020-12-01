// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addProcessToTask(x *xorm.Engine) error {
	type Task struct {
		Process string // process GUID and PID
	}

	return x.Sync2(new(Task))
}
