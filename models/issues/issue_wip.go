// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/models/system"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

// SyncIssueWIP backfills WIP flags on upgrade and when configured prefixes change, before serving requests.
func SyncIssueWIP(ctx context.Context) (string, error) {
	prefixes, err := json.Marshal(setting.Repository.PullRequest.WorkInProgressPrefixes)
	if err != nil {
		return "", err
	}
	const key = "issue_wip_prefixes"
	previous, exists, err := db.Get[system.Setting](ctx, builder.Eq{"setting_key": key})
	if err != nil || (exists && previous.SettingValue == string(prefixes)) {
		return string(prefixes), err
	}

	// Only load the fields needed for the backfill, not issue content.
	type Issue struct {
		ID    int64  `xorm:"pk"`
		Title string `xorm:"name"`
		IsWIP bool
	}
	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.Iterate(ctx, builder.Eq{"is_pull": true}, func(ctx context.Context, issue *Issue) error {
			isWIP := HasWorkInProgressPrefix(issue.Title)
			if issue.IsWIP == isWIP {
				return nil
			}
			_, err := db.GetEngine(ctx).ID(issue.ID).Cols("is_wip").NoAutoTime().Update(&Issue{IsWIP: isWIP})
			return err
		}); err != nil {
			return err
		}
		return system.SetSettings(ctx, map[string]string{key: string(prefixes)})
	})
	return string(prefixes), err
}
