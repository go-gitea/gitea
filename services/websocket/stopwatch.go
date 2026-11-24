// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/convert"
)

const eventStopwatches = "stopwatches" // keep in sync with web_src/js/types.ts

func (n *wsNotifier) StopwatchChanged(ctx context.Context, user *user_model.User) {
	publishUserEvent(user.ID, func() []byte { return stopwatchesEvent(ctx, user) })
}

func stopwatchesEvent(ctx context.Context, user *user_model.User) []byte {
	if !setting.Service.EnableTimetracking {
		return nil
	}
	sws, err := issues_model.GetUserStopwatches(ctx, user.ID, db.ListOptions{})
	if err != nil {
		log.Error("websocket: GetUserStopwatches %d: %v", user.ID, err)
		return nil
	}
	apiStopWatches, err := convert.ToStopWatches(ctx, user, sws)
	if err != nil {
		if !issues_model.IsErrIssueNotExist(err) {
			log.Error("websocket: ToStopWatches: %v", err)
		}
		return nil
	}
	return makeEventMessage(eventStopwatches, "", util.SliceNilAsEmpty(apiStopWatches))
}
