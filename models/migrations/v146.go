// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func addHookTaskPurge(x *xorm.Engine) error {
	type Repository struct {
		ID                            int64 `xorm:"pk autoincr"`
		IsHookTaskPurgeEnabled        bool  `xorm:"NOT NULL DEFAULT true"`
		NumberWebhookDeliveriesToKeep int64 `xorm:"NOT NULL DEFAULT 10"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE repository SET is_hook_task_purge_enabled = ?, number_webhook_deliveries_to_keep = ?",
		setting.Repository.DefaultIsHookTaskPurgeEnabled, setting.Repository.DefaultNumberWebhookDeliveriesToKeep)
	return err
}