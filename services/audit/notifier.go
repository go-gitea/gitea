// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"fmt"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(new(auditNotifier))
}

type auditNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = new(auditNotifier)

func issueLabel(issue *issues_model.Issue) string {
	return fmt.Sprintf("#%d", issue.Index)
}

func loadIssueRepo(ctx context.Context, issue *issues_model.Issue) *repo_model.Repository {
	if issue.Repo == nil {
		if err := issue.LoadRepo(ctx); err != nil {
			log.Error("audit: LoadRepo for issue %d: %v", issue.ID, err)
			return nil
		}
	}
	return issue.Repo
}

func (n *auditNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, _ []*user_model.User) {
	repo := loadIssueRepo(ctx, issue)
	if repo == nil {
		return
	}
	RecordAs(ctx, issue.Poster, audit_model.IssueCreate, repo,
		"issue", issueLabel(issue), "issue_id", issue.ID, "title", issue.Title)
}

func (n *auditNotifier) DeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	repo := loadIssueRepo(ctx, issue)
	if repo == nil {
		return
	}
	action := audit_model.IssueDelete
	key := "issue"
	if issue.IsPull {
		action = audit_model.PullRequestDelete
		key = "pull_request"
	}
	RecordAs(ctx, doer, action, repo, key, issueLabel(issue), "issue_id", issue.ID, "title", issue.Title)
}

func (n *auditNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, _ []*user_model.User) {
	if pr.Issue == nil {
		if err := pr.LoadIssue(ctx); err != nil {
			log.Error("audit: LoadIssue for pull request %d: %v", pr.ID, err)
			return
		}
	}
	repo := loadIssueRepo(ctx, pr.Issue)
	if repo == nil {
		return
	}
	RecordAs(ctx, pr.Issue.Poster, audit_model.PullRequestCreate, repo,
		"pull_request", issueLabel(pr.Issue), "pull_request_id", pr.ID, "title", pr.Issue.Title)
}

func (n *auditNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if pr.Issue == nil {
		if err := pr.LoadIssue(ctx); err != nil {
			log.Error("audit: LoadIssue for pull request %d: %v", pr.ID, err)
			return
		}
	}
	repo := loadIssueRepo(ctx, pr.Issue)
	if repo == nil {
		return
	}
	RecordAs(ctx, doer, audit_model.PullRequestMerge, repo,
		"pull_request", issueLabel(pr.Issue), "pull_request_id", pr.ID, "title", pr.Issue.Title)
}

func (n *auditNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, _ []*user_model.User) {
	action := audit_model.IssueCommentCreate
	key := "issue"
	if issue.IsPull {
		action = audit_model.PullRequestCommentCreate
		key = "pull_request"
	}
	RecordAs(ctx, doer, action, repo, key, issueLabel(issue), "comment_id", comment.ID)
}

func (n *auditNotifier) DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) {
	if comment.Issue == nil {
		if err := comment.LoadIssue(ctx); err != nil {
			log.Error("audit: LoadIssue for comment %d: %v", comment.ID, err)
			return
		}
	}
	repo := loadIssueRepo(ctx, comment.Issue)
	if repo == nil {
		return
	}
	action := audit_model.IssueCommentDelete
	key := "issue"
	if comment.Issue.IsPull {
		action = audit_model.PullRequestCommentDelete
		key = "pull_request"
	}
	RecordAs(ctx, doer, action, repo, key, issueLabel(comment.Issue), "comment_id", comment.ID)
}

func (n *auditNotifier) NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, _ string) {
	RecordAs(ctx, doer, audit_model.WikiPageCreate, repo, "page", page)
}

func (n *auditNotifier) EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, _ string) {
	RecordAs(ctx, doer, audit_model.WikiPageUpdate, repo, "page", page)
}

func (n *auditNotifier) DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	RecordAs(ctx, doer, audit_model.WikiPageDelete, repo, "page", page)
}
