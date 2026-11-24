// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/services/pubsub"
)

const eventPullHead = "pull-head" // keep in sync with web_src/js/types.ts

func pullTopic(pullID int64) string {
	return fmt.Sprintf("pull-%d", pullID)
}

func PublishPullHead(pullID int64) {
	pubsub.DefaultBroker.Publish(pullTopic(pullID), nil)
}

func pullTopicEventFunc(topic string) func(ctx context.Context, doerID int64) []byte {
	var pullID int64
	if _, err := fmt.Sscanf(topic, "pull-%d", &pullID); err != nil || topic != pullTopic(pullID) {
		return nil
	}
	return func(ctx context.Context, doerID int64) []byte {
		pull, err := issues_model.GetPullRequestByID(ctx, pullID)
		if err != nil || pull.LoadBaseRepo(ctx) != nil || !pull.BaseRepo.CanEnablePulls() {
			return nil
		}
		doer, err := user_model.GetValidDoerUserByID(ctx, doerID)
		if err != nil {
			return nil
		}
		if perm, err := access_model.GetDoerRepoPermission(ctx, pull.BaseRepo, doer); err != nil || !perm.CanRead(unit.TypePullRequests) {
			return nil
		}
		head, err := git.GetFullCommitID(ctx, pull.BaseRepo, pull.GetGitHeadRefName())
		if err != nil {
			return nil
		}
		return makeEventMessage(eventPullHead, "", struct {
			Topic string `json:"topic"`
			Head  string `json:"head"`
		}{topic, head})
	}
}
