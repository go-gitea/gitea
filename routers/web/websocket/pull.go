// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/services/pubsub"
	websocket_service "gitea.dev/services/websocket"

	gitea_ws "github.com/coder/websocket"
)

const maxPullHeadSubscriptions = 64

type pullHeadCommand struct {
	Type   string `json:"type"`
	PullID int64  `json:"pullId"`
}

func readPullHeadSubscriptions(ctx context.Context, cancelConnection context.CancelFunc, conn *gitea_ws.Conn, doer *user_model.User, needsTwoFactor bool, send func([]byte) error) {
	defer cancelConnection()
	subscriptions := make(map[int64]context.CancelFunc)
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var command pullHeadCommand
		if needsTwoFactor || messageType != gitea_ws.MessageText || json.Unmarshal(data, &command) != nil || command.PullID <= 0 {
			continue
		}
		switch command.Type {
		case "watch-pull-head":
			if cancel := subscriptions[command.PullID]; cancel != nil {
				cancel()
				delete(subscriptions, command.PullID)
			}
			if len(subscriptions) >= maxPullHeadSubscriptions {
				continue
			}
			pull, err := issues_model.GetPullRequestByID(ctx, command.PullID)
			if err != nil || pull.LoadIssue(ctx) != nil || pull.Issue.IsClosed {
				continue
			}
			baseRepo := readablePullRepo(ctx, doer, pull.BaseRepoID)
			if baseRepo == nil {
				continue
			}
			watchCtx, cancel := context.WithCancel(ctx)
			subscriptions[pull.ID] = cancel
			ch, unsubscribe := pubsub.DefaultBroker.Subscribe(pubsub.PullHeadTopic(pull.ID))
			go func() {
				defer unsubscribe()
				streamPullHead(watchCtx, cancelConnection, doer, pull, baseRepo, ch, send)
			}()
		case "unwatch-pull-head":
			if cancel := subscriptions[command.PullID]; cancel != nil {
				cancel()
				delete(subscriptions, command.PullID)
			}
		}
	}
}

func readablePullRepo(ctx context.Context, doer *user_model.User, repoID int64) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil || !repo.CanEnablePulls() {
		return nil
	}
	currentDoer, err := user_model.GetValidDoerUserByID(ctx, doer.ID)
	if err != nil {
		return nil
	}
	currentDoer.ExtDoerData = doer.ExtDoerData
	perm, err := access_model.GetDoerRepoPermission(ctx, repo, currentDoer)
	if err != nil || !perm.CanRead(unit.TypePullRequests) {
		return nil
	}
	return repo
}

func streamPullHead(ctx context.Context, cancelConnection context.CancelFunc, doer *user_model.User, pull *issues_model.PullRequest, baseRepo *repo_model.Repository, ch <-chan []byte, send func([]byte) error) {
	for len(ch) > 0 {
		<-ch // The initial ref read supersedes previously queued updates.
	}
	head, err := git.GetFullCommitID(ctx, baseRepo, pull.GetGitHeadRefName())
	if ctx.Err() != nil {
		return
	}
	if err == nil && send(websocket_service.PullHeadEvent(pull.ID, head)) != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-ch:
			if ctx.Err() != nil {
				return
			}
			if !ok {
				cancelConnection()
				return
			}
			if readablePullRepo(ctx, doer, pull.BaseRepoID) == nil || send(payload) != nil {
				return
			}
		}
	}
}
