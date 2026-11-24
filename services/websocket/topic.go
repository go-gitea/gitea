// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/services/pubsub"
)

const eventPullHead = "pull-head" // keep in sync with web_src/js/types.ts

// Topic is a pubsub topic that pages subscribe to over the WebSocket, its messages only signal that CurrentEvent changed
type Topic interface {
	CanRead(ctx context.Context) bool
	CurrentEvent(ctx context.Context) []byte
}

func GetDoerTopic(ctx context.Context, doerID int64, name string) Topic {
	var pullID int64
	if _, err := fmt.Sscanf(name, "pull-%d", &pullID); err != nil || name != pubsub.PullTopic(pullID) {
		return nil
	}
	pull, err := issues_model.GetPullRequestByID(ctx, pullID)
	if err != nil {
		return nil
	}
	topic := &pullTopic{doerID: doerID, pull: pull}
	if !topic.CanRead(ctx) {
		return nil
	}
	return topic
}

type pullTopic struct {
	doerID int64
	pull   *issues_model.PullRequest
}

func (t *pullTopic) CanRead(ctx context.Context) bool {
	repo, err := repo_model.GetRepositoryByID(ctx, t.pull.BaseRepoID)
	if err != nil || !repo.CanEnablePulls() {
		return false
	}
	t.pull.BaseRepo = repo
	doer, err := user_model.GetValidDoerUserByID(ctx, t.doerID) // reloaded because the connection outlives changes to the doer
	if err != nil {
		return false
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
	return err == nil && perm.CanRead(unit.TypePullRequests)
}

func (t *pullTopic) CurrentEvent(ctx context.Context) []byte {
	head, err := git.GetFullCommitID(ctx, t.pull.BaseRepo, t.pull.GetGitHeadRefName())
	if err != nil {
		return nil
	}
	return makeEventMessage(eventPullHead, "", struct {
		PullID int64  `json:"pullId"`
		Head   string `json:"head"`
	}{t.pull.ID, head})
}
