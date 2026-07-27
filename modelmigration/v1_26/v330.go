// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddNameToWebhook(x base.EngineMigration) error {
	type Webhook struct {
		Name string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Webhook))
	return err
}
