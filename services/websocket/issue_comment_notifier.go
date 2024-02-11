// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"github.com/olahol/melody"
)

func (n *websocketNotifier) filterIssueSessions(repo *repo_model.Repository, issue *issues_model.Issue) []*melody.Session {
	return n.filterSessions(func(s *melody.Session, data *sessionData) bool {
		// if the user is watching the issue, they will get notifications
		if !data.isOnURL(fmt.Sprintf("/%s/%s/issues/%d", repo.Owner.Name, repo.Name, issue.Index)) {
			return false
		}

		// if the repo is public, the user will get notifications
		if !repo.IsPrivate {
			return true
		}

		// if the repo is private, the user will get notifications if they have access to the repo

		// TODO: check if the user has access to the repo
		return data.userID == issue.PosterID
	})
}

func (n *websocketNotifier) DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	sessions := n.filterIssueSessions(c.Issue.Repo, c.Issue)

	for _, s := range sessions {
		msg := fmt.Sprintf(htmxRemoveElement, fmt.Sprintf("#%s", c.HashTag()))
		err := s.Write([]byte(msg))
		if err != nil {
			log.Error("Failed to write to session: %v", err)
		}
	}
}
