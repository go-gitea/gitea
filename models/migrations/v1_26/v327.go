// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"time"

	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

// pushCommitsMigration represents commits in a push action (simplified for migration)
type pushCommitsMigration struct {
	Commits []*pushCommitMigration `json:"commits"`
}

// pushCommitMigration represents a commit in a push (simplified for migration)
type pushCommitMigration struct {
	Sha1      string    `json:"sha1"`
	Timestamp time.Time `json:"timestamp"`
}

func BackfillUserHeatmapCommits(x *xorm.Engine) error {
	const batchSize = 100
	const actionCommitRepo = 5      // ActionCommitRepo operation type
	const actionMirrorSyncPush = 16 // ActionMirrorSyncPush operation type

	// Process actions in batches
	var lastID int64
	for {
		type ActionRow struct {
			ID        int64  `xorm:"id"`
			ActUserID int64  `xorm:"act_user_id"`
			RepoID    int64  `xorm:"repo_id"`
			Content   string `xorm:"content"`
		}

		actions := make([]*ActionRow, 0, batchSize)
		err := x.Table("action").
			Select("id, act_user_id, repo_id, content").
			Where("op_type = ? OR op_type = ?", actionCommitRepo, actionMirrorSyncPush).
			And("id > ?", lastID).
			And("content != ''").
			OrderBy("id").
			Limit(batchSize).
			Find(&actions)
		if err != nil {
			return err
		}

		if len(actions) == 0 {
			break
		}

		for _, action := range actions {
			var pushCommits pushCommitsMigration
			if err := json.Unmarshal([]byte(action.Content), &pushCommits); err != nil {
				// Skip actions with invalid JSON
				continue
			}

			if len(pushCommits.Commits) == 0 {
				continue
			}

			records := make([]map[string]any, 0, len(pushCommits.Commits))
			for _, commit := range pushCommits.Commits {
				timestamp := commit.Timestamp.Unix()
				if timestamp <= 0 {
					continue
				}

				records = append(records, map[string]any{
					"user_id":          action.ActUserID,
					"repo_id":          action.RepoID,
					"commit_sha1":      commit.Sha1,
					"commit_timestamp": timestamp,
				})
			}

			if len(records) > 0 {
				if _, err := x.Table("user_heatmap_commit").Insert(&records); err != nil {
					return err
				}
			}
		}

		lastID = actions[len(actions)-1].ID
	}

	return nil
}
