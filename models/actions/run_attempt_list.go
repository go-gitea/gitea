// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

type ActionRunAttemptList []*ActionRunAttempt

// GetUserIDs returns a slice of user's id
func (attempts ActionRunAttemptList) GetUserIDs() []int64 {
	return container.FilterSlice(attempts, func(attempt *ActionRunAttempt) (int64, bool) {
		return attempt.TriggerUserID, true
	})
}

func (attempts ActionRunAttemptList) LoadTriggerUser(ctx context.Context) error {
	userIDs := attempts.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, attempt := range attempts {
		if attempt.TriggerUserID == user_model.ActionsUserID {
			attempt.TriggerUser = user_model.NewActionsUser()
		} else {
			attempt.TriggerUser = users[attempt.TriggerUserID]
			if attempt.TriggerUser == nil {
				attempt.TriggerUser = user_model.NewGhostUser()
			}
		}
	}
	return nil
}

// ListRunAttemptsByRunID returns all attempts of a run, ordered by attempt number DESC (newest first).
func ListRunAttemptsByRunID(ctx context.Context, runID int64) (ActionRunAttemptList, error) {
	var attempts ActionRunAttemptList
	return attempts, db.GetEngine(ctx).Where("run_id=?", runID).OrderBy("attempt DESC").Find(&attempts)
}
