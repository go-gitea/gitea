// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

// CommitStatus see models/status.go
type CommitStatus struct {
	ID          int64  `xorm:"pk autoincr"`
	Index       int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
	RepoID      int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
	State       string `xorm:"VARCHAR(7) NOT NULL"`
	SHA         string `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	TargetURL   string `xorm:"TEXT"`
	Description string `xorm:"TEXT"`
	Context     string `xorm:"TEXT"`
	CreatorID   int64  `xorm:"INDEX"`

	CreatedUnix int64 `xorm:"INDEX"`
	UpdatedUnix int64 `xorm:"INDEX"`
}

func addCommitStatus(x *xorm.Engine) error {
	if err := x.Sync2(new(CommitStatus)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
