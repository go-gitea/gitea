// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
	"github.com/olahol/melody"
)

type webhookNotifier struct {
	notify_service.NullNotifier
	m *melody.Melody
}

var (
	_               notify_service.Notifier = &webhookNotifier{}
	tplIssueComment base.TplName            = "repo/issue/view"
)

// NewNotifier create a new webhooksNotifier notifier
func NewNotifier(m *melody.Melody) notify_service.Notifier {
	return &webhookNotifier{
		m: m,
	}
}

var addElementHTML = "<div hx-swap-oob=\"beforebegin:%s\">%s</div>"

func (n *webhookNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
	// TODO: use proper message
	var content bytes.Buffer

	tmpl := new(template.Template)
	if err := tmpl.ExecuteTemplate(&content, string(tplIssueComment), comment); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := fmt.Sprintf(addElementHTML, ".timeline-item.comment.form", "test")

	err := n.m.BroadcastFilter([]byte(msg), func(s *melody.Session) bool {
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
