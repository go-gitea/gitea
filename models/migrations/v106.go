// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

// RepoWatchMode specifies what kind of watch the user has on a repository
type RepoWatchMode int8

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID   int64         `xorm:"pk autoincr"`
	Mode RepoWatchMode `xorm:"SMALLINT NOT NULL DEFAULT 1"`
}

func addModeColumnToWatch(x *xorm.Engine) (err error) {
	if err = x.Sync2(new(Watch)); err != nil {
		return
	}
	_, err = x.Exec("UPDATE `watch` SET `mode` = 1")
	return err
}
