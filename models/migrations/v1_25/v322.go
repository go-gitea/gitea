// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"xorm.io/xorm"
)

func AddWebhookPayloadOptimizationColumns(x *xorm.Engine) error {
	type Webhook struct {
		ExcludeFilesLimit   int `xorm:"exclude_files_limit NOT NULL DEFAULT -1"`
		ExcludeCommitsLimit int `xorm:"exclude_commits_limit NOT NULL DEFAULT -1"`
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
