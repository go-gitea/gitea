// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"github.com/olahol/melody"
)

func (n *websocketNotifier) filterIssueSessions(ctx context.Context, repo *repo_model.Repository, issue *issues_model.Issue) []*melody.Session {
	return n.filterSessions(func(s *melody.Session, data *sessionData) bool {
		// if the user is watching the issue, they will get notifications
		if !data.isOnURL(fmt.Sprintf("/%s/%s/issues/%d", repo.Owner.Name, repo.Name, issue.Index)) {
			return false
		}

		// the user will get notifications if they have access to the repos issues
		hasAccess, err := access.HasAccessUnit(ctx, data.user, repo, unit.TypeIssues, perm.AccessModeRead)
		if err != nil {
			log.Error("Failed to check access: %v", err)
			return false
		}

		return hasAccess
	})
}

func (n *websocketNotifier) DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	sessions := n.filterIssueSessions(ctx, c.Issue.Repo, c.Issue)

	for _, s := range sessions {
		msg := fmt.Sprintf(htmxRemoveElement, fmt.Sprintf("#%s", c.HashTag()))
		err := s.Write([]byte(msg))
		if err != nil {
			log.Error("Failed to write to session: %v", err)
		}
	}
}
