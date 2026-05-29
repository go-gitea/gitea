// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/log"

	"xorm.io/builder"
)

func UpdateMatrixWebhookHTTPMethod(x db.EngineMigration) error {
	matrixHookTaskType := 9 // value comes from the models package
	type Webhook struct {
		HTTPMethod string
	}

	cond := builder.Eq{"hook_task_type": matrixHookTaskType}.And(builder.Neq{"http_method": "PUT"})
	count, err := x.Where(cond).Cols("http_method").Update(&Webhook{HTTPMethod: "PUT"})
	if err == nil {
		log.Debug("Updated %d Matrix webhooks with http_method 'PUT'", count)
	}
	return err
}
