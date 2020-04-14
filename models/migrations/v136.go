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
		EnableHookTaskPurge           bool  `xorm:"NOT NULL DEFAULT true"`
		NumberWebhookDeliveriesToKeep int64 `xorm:"NOT NULL DEFAULT 10"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE repository SET enable_hook_task_purge = ?", number_webhook_deliveries_to_keep = ?
		setting.Repository.DefaultEnableHookTaskPurge, setting.Repository.DefaultNumberWebhookDeliveriesToKeep)
	return err
}
