// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

// AddGitSizeAndLFSSizeToRepositoryTable: add GitSize and LFSSize columns to Repository
func AddGitSizeAndLFSSizeToRepositoryTable(x *xorm.Engine) error {
	type Repository struct {
		GitSize int64 `xorm:"NOT NULL DEFAULT 0"`
		LFSSize int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(Repository)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}

	_, err := sess.Exec(`UPDATE repository SET lfs_size=(SELECT SUM(size) FROM lfs_meta_object WHERE lfs_meta_object.repository_id=repository.ID) WHERE EXISTS (SELECT 1 FROM lfs_meta_object WHERE lfs_meta_object.repository_id=repository.ID)`)
	if err != nil {
		return err
	}

	_, err = sess.Exec(`UPDATE repository SET git_size = size - lfs_size`)
	if err != nil {
		return err
	}

	return sess.Commit()
}
