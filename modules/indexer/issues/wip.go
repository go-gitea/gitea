// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"crypto/sha256"
	"fmt"

	"gitea.dev/models/db"
	"gitea.dev/models/system"
	"gitea.dev/modules/indexer/issues/internal"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

func syncWIPIndexer(ctx context.Context, indexer internal.Indexer, prefixes string, existed bool) error {
	if setting.Indexer.IssueType == "db" {
		return nil
	}
	identity := fmt.Sprintf("%s\n%s\n%s\n%s", setting.Indexer.IssueType, setting.Indexer.IssuePath, setting.Indexer.IssueConnStr, setting.Indexer.IssueIndexerName)
	key := fmt.Sprintf("issue_wip_indexer_%x", sha256.Sum256([]byte(identity)))
	previous, found, err := db.Get[system.Setting](ctx, builder.Eq{"setting_key": key})
	if err != nil || (existed && found && previous.SettingValue == prefixes) {
		return err
	}
	if existed {
		type Issue struct {
			ID int64 `xorm:"pk"`
		}
		// Complete the refresh before recording the prefixes so interrupted refreshes retry on restart.
		if err := db.Iterate(ctx, builder.Eq{"is_pull": true}, func(ctx context.Context, issue *Issue) error {
			data, exists, err := getIssueIndexerData(ctx, issue.ID)
			if err != nil || !exists {
				return err
			}
			return indexer.Index(ctx, data)
		}); err != nil {
			return err
		}
	}
	return system.SetSettings(ctx, map[string]string{key: prefixes})
}
