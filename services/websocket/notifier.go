// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
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

type websocketNotifier struct {
	notify_service.NullNotifier
	m   *melody.Melody
	rnd *templates.HTMLRender
}

var tplIssueComment base.TplName = "repo/issue/view_content/comment"

// NewNotifier create a new webhooksNotifier notifier
func NewNotifier(m *melody.Melody) notify_service.Notifier {
	return &websocketNotifier{
		m:   m,
		rnd: templates.HTMLRenderer(),
	}
}

var (
	htmxAddElementEnd = "<div hx-swap-oob=\"beforebegin:%s\">%s</div>"
	// htmxUpdateElement = "<div hx-swap-oob=\"outerHTML:%s\">%s</div>"
	htmxRemoveElement = "<div hx-swap-oob=\"delete:%s\"></div>"
)

func (n *websocketNotifier) filterSessions(fn func(*melody.Session, *sessionData) bool) []*melody.Session {
	sessions, err := n.m.Sessions()
	if err != nil {
		log.Error("Failed to get sessions: %v", err)
		return nil
	}

	_sessions := make([]*melody.Session, 0, len(sessions))
	for _, s := range sessions {
		data, err := getSessionData(s)
		if err != nil {
			continue
		}

		if fn(s, data) {
			_sessions = append(_sessions, s)
		}
	}

	return _sessions
}

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

func (n *websocketNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
	sessions := n.filterIssueSessions(repo, issue)

	for _, s := range sessions {
		var content bytes.Buffer

		webCtx := web_context.GetWebContext(s.Request)
		webCtx.Repo.Repository = repo

		t, err := webCtx.Render.TemplateLookup(string(tplIssueComment), webCtx.TemplateContext)
		if err != nil {
			log.Error("Failed to lookup template: %v", err)
			return
		}

		if err := comment.LoadPoster(ctx); err != nil {
			log.Error("Failed to load comment poster: %v", err)
			return
		}

		if comment.Type == issues_model.CommentTypeComment || comment.Type == issues_model.CommentTypeReview {
			if err := comment.LoadAttachments(ctx); err != nil {
				log.Error("Failed to load comment attachments: %v", err)
				return
			}

			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				Links: markup.Links{
					Base: webCtx.Repo.RepoLink,
				},
				Metas:   webCtx.Repo.Repository.ComposeMetas(ctx),
				GitRepo: webCtx.Repo.GitRepo,
				Ctx:     webCtx,
			}, comment.Content)
			if err != nil {
				log.Error("Failed to render comment content: %v", err)
				return
			}
			comment.ShowRole, err = roleDescriptor(ctx, repo, comment.Poster, issue, comment.HasOriginalAuthor())
			if err != nil {
				log.Error("Failed to get role descriptor: %v", err)
				return
			}

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

		msg := fmt.Sprintf(htmxAddElementEnd, ".timeline-item.comment.form", content.String())
		err = s.Write([]byte(msg))
		if err != nil {
			log.Error("Failed to write to session: %v", err)
		}
	}
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

// roleDescriptor returns the role descriptor for a comment in/with the given repo, poster and issue
func roleDescriptor(ctx context.Context, repo *repo_model.Repository, poster *user_model.User, issue *issues_model.Issue, hasOriginalAuthor bool) (issues_model.RoleDescriptor, error) {
	roleDescriptor := issues_model.RoleDescriptor{}

	if hasOriginalAuthor {
		return roleDescriptor, nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, poster)
	if err != nil {
		return roleDescriptor, err
	}

	// If the poster is the actual poster of the issue, enable Poster role.
	roleDescriptor.IsPoster = issue.IsPoster(poster.ID)

	// Check if the poster is owner of the repo.
	if perm.IsOwner() {
		// If the poster isn't an admin, enable the owner role.
		if !poster.IsAdmin {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoOwner
			return roleDescriptor, nil
		}

		// Otherwise check if poster is the real repo admin.
		ok, err := access_model.IsUserRealRepoAdmin(ctx, repo, poster)
		if err != nil {
			return roleDescriptor, err
		}
		if ok {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoOwner
			return roleDescriptor, nil
		}
	}

	// If repo is organization, check Member role
	if err := repo.LoadOwner(ctx); err != nil {
		return roleDescriptor, err
	}
	if repo.Owner.IsOrganization() {
		if isMember, err := organization.IsOrganizationMember(ctx, repo.Owner.ID, poster.ID); err != nil {
			return roleDescriptor, err
		} else if isMember {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoMember
			return roleDescriptor, nil
		}
	}

	// If the poster is the collaborator of the repo
	if isCollaborator, err := repo_model.IsCollaborator(ctx, repo.ID, poster.ID); err != nil {
		return roleDescriptor, err
	} else if isCollaborator {
		roleDescriptor.RoleInRepo = issues_model.RoleRepoCollaborator
		return roleDescriptor, nil
	}

	hasMergedPR, err := issues_model.HasMergedPullRequestInRepo(ctx, repo.ID, poster.ID)
	if err != nil {
		return roleDescriptor, err
	} else if hasMergedPR {
		roleDescriptor.RoleInRepo = issues_model.RoleRepoContributor
	} else if issue.IsPull {
		// only display first time contributor in the first opening pull request
		roleDescriptor.RoleInRepo = issues_model.RoleRepoFirstTimeContributor
	}

	return roleDescriptor, nil
}
