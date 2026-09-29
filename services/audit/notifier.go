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

func (n *auditNotifier) CreateRepository(ctx context.Context, doer, _ *user_model.User, repo *repo_model.Repository) {
	RecordAs(ctx, doer, audit_model.RepositoryCreate, repo)
}

func (n *auditNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	RecordAs(ctx, doer, audit_model.RepositoryCreateFork, repo, "base_repo", oldRepo.FullName())
}

func (n *auditNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
	RecordAs(ctx, doer, audit_model.RepositoryName, repo, "previous_name", oldRepoName)
}

func (n *auditNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
	RecordAs(ctx, doer, audit_model.RepositoryTransferFinish, repo, "old_owner", oldOwnerName, "new_owner", repo.OwnerName)
}

func (n *auditNotifier) RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	RecordAs(ctx, doer, audit_model.RepositoryTransferStart, repo, "new_owner", newOwner.Name)
}

func (n *auditNotifier) ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
	Record(ctx, audit_model.RepositoryBranchDefault, repo, "default_branch", repo.DefaultBranch)
}

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

func issueOrPR(issue *issues_model.Issue, issueAction, prAction audit_model.Action) (audit_model.Action, string) {
	if issue.IsPull {
		return prAction, "pull_request"
	}
	return issueAction, "issue"
}

// issueMeta keeps the label, ID and title keys of every issue and pull request
// event consistent; the ID is always the issue row ID the label refers to.
func issueMeta(issue *issues_model.Issue, key string) []any {
	return []any{key, issueLabel(issue), key + "_id", issue.ID, "title", issue.Title}
}

func (n *auditNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, _ []*user_model.User) {
	repo := loadIssueRepo(ctx, issue)
	if repo == nil {
		return
	}
	RecordAs(ctx, issue.Poster, audit_model.IssueCreate, repo, issueMeta(issue, "issue")...)
}

func (n *auditNotifier) DeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	repo := loadIssueRepo(ctx, issue)
	if repo == nil {
		return
	}
	action, key := issueOrPR(issue, audit_model.IssueDelete, audit_model.PullRequestDelete)
	RecordAs(ctx, doer, action, repo, issueMeta(issue, key)...)
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
	RecordAs(ctx, pr.Issue.Poster, audit_model.PullRequestCreate, repo, issueMeta(pr.Issue, "pull_request")...)
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
	RecordAs(ctx, doer, audit_model.PullRequestMerge, repo, issueMeta(pr.Issue, "pull_request")...)
}

func (n *auditNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, _ []*user_model.User) {
	action, key := issueOrPR(issue, audit_model.IssueCommentCreate, audit_model.PullRequestCommentCreate)
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
	action, key := issueOrPR(comment.Issue, audit_model.IssueCommentDelete, audit_model.PullRequestCommentDelete)
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
