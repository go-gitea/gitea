// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addRepoIndexerStatus(x *xorm.Engine) error {
	// RepoIndexerStatus see models/repo_indexer.go
	type RepoIndexerStatus struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"INDEX NOT NULL"`
		CommitSha string `xorm:"VARCHAR(40)"`
	}

	if err := x.Sync2(new(RepoIndexerStatus)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
