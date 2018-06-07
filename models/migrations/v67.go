// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package migrations

import (
	"github.com/go-xorm/xorm"
)

func cleanUnlinkedWebhookAndHookTasks(x *xorm.Engine) error {
	_, err := x.Exec(`DELETE FROM webhook WHERE (SELECT COUNT(*) FROM repository WHERE id = webhook.repo_id)=0;DELETE FROM hook_task WHERE (SELECT COUNT(*) FROM repository WHERE id = hook_task.repo_id)=0;`)
	return err
}
