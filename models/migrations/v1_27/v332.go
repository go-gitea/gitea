// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

// AddMirrorSyncStatus adds persisted sync status fields for pull and push mirrors.
func AddMirrorSyncStatus(x *xorm.Engine) error {
	type Mirror struct {
		IsSynced bool `xorm:"NOT NULL DEFAULT false"`
	}

	type PushMirror struct {
		IsSynced bool `xorm:"NOT NULL DEFAULT false"`
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices: true,
	}, new(Mirror), new(PushMirror)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE mirror SET is_synced = ? WHERE repo_id IN (SELECT id FROM repository WHERE is_mirror = ?)", true, true)
	return err
}
