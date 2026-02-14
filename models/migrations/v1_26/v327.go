// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"time"

	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

// PushCommits represents commits in a push action (simplified for migration)
type PushCommits struct {
	Commits []*PushCommit `json:"commits"`
}

// PushCommit represents a commit in a push (simplified for migration)
type PushCommit struct {
	Sha1      string    `json:"sha1"`
	Timestamp time.Time `json:"timestamp"`
}

func BackfillActionCommitDates(x *xorm.Engine) error {
	const batchSize = 100
	const actionCommitRepo = 5 // ActionCommitRepo operation type

	// Only backfill actions within the heatmap window (373 days = 366 + 7 days buffer)
	// Older actions won't be displayed in the heatmap anyway
	cutoff := time.Now().AddDate(0, 0, -373).Unix()

	// Process actions in batches
	var lastID int64
	for {
		// Query batch of recent push actions only
		type ActionRow struct {
			ID      int64  `xorm:"id"`
			Content string `xorm:"content"`
		}

		actions := make([]*ActionRow, 0, batchSize)
		err := x.Table("action").
			Select("id, content").
			Where("op_type = ?", actionCommitRepo).
			And("id > ?", lastID).
			And("created_unix > ?", cutoff).
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

		// Process each action
		for _, action := range actions {
			// Parse commits from JSON
			var pushCommits PushCommits
			if err := json.Unmarshal([]byte(action.Content), &pushCommits); err != nil {
				// Skip actions with invalid JSON (might be empty or different format)
				continue
			}

			if len(pushCommits.Commits) == 0 {
				continue
			}

			// Insert commit date records, skipping invalid timestamps
			commitDates := make([]map[string]any, 0, len(pushCommits.Commits))
			for _, commit := range pushCommits.Commits {
				timestamp := commit.Timestamp.Unix()
				// Skip zero-value or negative timestamps (would be nonsensical contributions)
				if timestamp <= 0 {
					continue
				}

				commitDates = append(commitDates, map[string]any{
					"action_id":        action.ID,
					"commit_sha1":      commit.Sha1,
					"commit_timestamp": timestamp,
				})
			}

			if len(commitDates) > 0 {
				if _, err := x.Table("action_commit_date").Insert(&commitDates); err != nil {
					return err
				}
			}
		}

		lastID = actions[len(actions)-1].ID
	}

	return nil
}
