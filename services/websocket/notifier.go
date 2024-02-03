// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
	"github.com/olahol/melody"
)

type webhookNotifier struct {
	notify_service.NullNotifier
	m *melody.Melody
}

var _ notify_service.Notifier = &webhookNotifier{}

// NewNotifier create a new webhooksNotifier notifier
func NewNotifier(m *melody.Melody) notify_service.Notifier {
	return &webhookNotifier{
		m: m,
	}
}

func (n *webhookNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
	// TODO: use proper message
	msg := []byte("<div hx-swap-oob=\"beforebegin:.timeline-item.comment.form\"><div class=\"hello\">hello world!</div></div>")

	err := n.m.BroadcastFilter(msg, func(s *melody.Session) bool {
		sessionData, err := getSessionData(s)
		if err != nil {
			return false
		}

		if sessionData.uid == doer.ID {
			return true
		}

		for _, mention := range mentions {
			if mention.ID == sessionData.uid {
				return true
			}
		}

		return false
	})
	if err != nil {
		log.Error("Failed to broadcast message: %v", err)
	}
}
