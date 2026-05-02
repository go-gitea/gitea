// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/actions"
	api "code.gitea.io/gitea/modules/structs"
)

// ReadStepLogs reads log lines for the given cursor positions from a task.
// expiredMessage is used as the log content when the task's logs have expired.
func ReadStepLogs(ctx context.Context, cursors []api.ActionLogCursor, task *actions_model.ActionTask, expiredMessage string) ([]*api.ActionLogStep, error) {
	var logs []*api.ActionLogStep
	steps := actions.FullSteps(task)

	for _, cursor := range cursors {
		if !cursor.Expanded {
			continue
		}
		if cursor.Step >= len(steps) {
			continue
		}
		step := steps[cursor.Step]

		if task.LogExpired {
			if cursor.Cursor == 0 {
				logs = append(logs, &api.ActionLogStep{
					Step:   cursor.Step,
					Cursor: 1,
					Lines: []*api.ActionLogStepLine{{
						Index:     1,
						Message:   expiredMessage,
						Timestamp: float64(task.Updated.AsTime().UnixNano()) / float64(time.Second),
					}},
					Started: int64(step.Started),
				})
			}
			continue
		}

		logLines := make([]*api.ActionLogStepLine, 0)
		index := step.LogIndex + cursor.Cursor
		validCursor := cursor.Cursor >= 0 &&
			// !(cursor.Cursor < step.LogLength) when the frontend tries to fetch the next
			// line before it's ready — return same cursor and empty lines to let caller retry.
			cursor.Cursor < step.LogLength &&
			// !(index < len(task.LogIndexes)) when task data is older than step data.
			index < int64(len(task.LogIndexes))

		if validCursor {
			length := step.LogLength - cursor.Cursor
			offset := task.LogIndexes[index]
			logRows, err := actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, offset, length)
			if err != nil {
				return nil, fmt.Errorf("actions.ReadLogs: %w", err)
			}
			for i, row := range logRows {
				logLines = append(logLines, &api.ActionLogStepLine{
					Index:     cursor.Cursor + int64(i) + 1, // 1-based
					Message:   row.Content,
					Timestamp: float64(row.Time.AsTime().UnixNano()) / float64(time.Second),
				})
			}
		}

		logs = append(logs, &api.ActionLogStep{
			Step:    cursor.Step,
			Cursor:  cursor.Cursor + int64(len(logLines)),
			Lines:   logLines,
			Started: int64(step.Started),
		})
	}
	return logs, nil
}
