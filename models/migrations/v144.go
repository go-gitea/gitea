// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func updateMatrixWebhookHTTPMethod(x *xorm.Engine) error {
	type Webhook struct {
		HTTPMethod string
	}
	count, err := x.Where(
		builder.Neq{
			"http_method":    "PUT",
			"hook_task_type": models.MATRIX,
		}).Cols("http_method").Update(&Webhook{HTTPMethod: "PUT"})
	if err == nil {
		log.Debug("Updated %d Matrix webhooks with http_method 'PUT'", count)
	}
	return err
}
