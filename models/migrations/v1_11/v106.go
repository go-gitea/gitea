// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

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

func AddModeColumnToWatch(x *xorm.Engine) error {
	if err := x.Sync(new(Watch)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE `watch` SET `mode` = 1")
	return err
}
