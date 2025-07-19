// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"xorm.io/xorm"
)

func AddWebhookPayloadOptimizationColumns(x *xorm.Engine) error {
	type Webhook struct {
		ExcludeFiles   bool `xorm:"exclude_files NOT NULL DEFAULT false"`
		ExcludeCommits bool `xorm:"exclude_commits NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(
		xorm.SyncOptions{
			IgnoreConstrains: true,
			IgnoreIndices:    true,
		},
		new(Webhook),
	)
	return err
}
