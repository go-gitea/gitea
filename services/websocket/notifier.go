// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	web_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/templates"
	notify_service "code.gitea.io/gitea/services/notify"
	"github.com/olahol/melody"
)

type webhookNotifier struct {
	notify_service.NullNotifier
	m   *melody.Melody
	rnd *templates.HTMLRender
}

var tplIssueComment base.TplName = "repo/issue/view_content/comment"

// NewNotifier create a new webhooksNotifier notifier
func NewNotifier(m *melody.Melody) notify_service.Notifier {
	return &webhookNotifier{
		m:   m,
		rnd: templates.HTMLRenderer(),
	}
}

var addElementHTML = "<div hx-swap-oob=\"beforebegin:%s\">%s</div>"

func (n *webhookNotifier) filterSessions(fn func(*melody.Session) bool) []*melody.Session {
	sessions, err := n.m.Sessions()
	if err != nil {
		log.Error("Failed to get sessions: %v", err)
		return nil
	}

	_sessions := make([]*melody.Session, 0, len(sessions))
	for _, s := range sessions {
		if fn(s) {
			_sessions = append(_sessions, s)
		}
	}

	return _sessions
}

func (n *webhookNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
	sessions := n.filterSessions(func(s *melody.Session) bool {
		sessionData, err := getSessionData(s)
		if err != nil {
			return false
		}

		if sessionData.uid == doer.ID {
			return true
		}

		for _, mention := range mentions {
			if sessionData.uid == mention.ID {
				return true
			}
		}

		return false
	})

	for _, s := range sessions {
		var content bytes.Buffer

		webCtx := web_context.GetWebContext(s.Request)

		t, err := webCtx.Render.TemplateLookup(string(tplIssueComment), webCtx.TemplateContext)
		if err != nil {
			log.Error("Failed to lookup template: %v", err)
			return
		}

		issue.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			Links: markup.Links{
				Base: webCtx.Repo.RepoLink,
			},
			Metas:   repo.ComposeMetas(ctx),
			GitRepo: webCtx.Repo.GitRepo,
			Ctx:     ctx,
		}, issue.Content)
		if err != nil {
			log.Error("Failed to render issue content: %v", err)
			return
		}

		ctxData := map[string]any{}
		ctxData["Repository"] = repo
		ctxData["Issue"] = issue
		ctxData["IsSigned"] = true

		data := map[string]any{}
		data["ctxData"] = ctxData
		data["Comment"] = comment

		if err := t.Execute(&content, data); err != nil {
			log.Error("Template: %v", err)
			return
		}

		msg := fmt.Sprintf(addElementHTML, ".timeline-item.comment.form", content.String())
		err = s.Write([]byte(msg))
		if err != nil {
			log.Error("Failed to write to session: %v", err)
		}

	}
}
