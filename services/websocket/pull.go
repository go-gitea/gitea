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
	"gitea.dev/modules/json"
	"gitea.dev/services/pubsub"
)

const eventPullHead = "pull-head" // keep in sync with web_src/js/types.ts

type pullHeadMessage struct { // keep in sync with web_src/js/features/repo-diff.ts
	PullID int64 `json:"pullID"`
}

func pullTopic(pullID int64) string {
	return fmt.Sprintf("pull-head-%d", pullID)
}

func PublishPullHead(pullID int64) {
	pubsub.DefaultBroker.Publish(pullTopic(pullID), nil)
}

func pullHeadSubscription(doerID int64, msgJSON []byte) (string, func(context.Context) []byte) {
	var msg pullHeadMessage
	if json.Unmarshal(msgJSON, &msg) != nil {
		return "", nil
	}
	return pullTopic(msg.PullID), func(ctx context.Context) []byte { return pullHeadEvent(ctx, doerID, msg.PullID) }
}

func pullHeadEvent(ctx context.Context, doerID, pullID int64) []byte {
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
		PullID int64  `json:"pullID"`
		Head   string `json:"head"`
	}{pullID, head})
}
