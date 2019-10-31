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
	Mode RepoWatchMode `xorm:"SMALLINT NOT NULL DEFAULT 1"`
}

func addModeColumnToWatch(x *xorm.Engine) error {
	return x.Sync2(new(Watch))
}
