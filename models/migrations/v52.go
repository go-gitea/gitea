// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func addLFSLock(x *xorm.Engine) error {
	// LFSLock see models/lfs_lock.go
	type LFSLock struct {
		ID      int64        `xorm:"pk autoincr"`
		RepoID  int64        `xorm:"INDEX NOT NULL"`
		Owner   *models.User `xorm:"-"`
		OwnerID int64        `xorm:"INDEX NOT NULL"`
		Path    string       `xorm:"TEXT"`
		Created time.Time    `xorm:"created"`
	}

	if err := x.Sync2(new(LFSLock)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
