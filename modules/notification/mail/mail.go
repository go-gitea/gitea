// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mail

import (
	"context"
	"fmt"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/services/mailer"
)

type mailNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &mailNotifier{}

// NewNotifier create a new mailNotifier notifier
func NewNotifier() base.Notifier {
	return &mailNotifier{}
}

func (m *mailNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	var act activities_model.ActionType
	if comment.Type == issues_model.CommentTypeClose {
		act = activities_model.ActionCloseIssue
	} else if comment.Type == issues_model.CommentTypeReopen {
		act = activities_model.ActionReopenIssue
	} else if comment.Type == issues_model.CommentTypeComment {
		act = activities_model.ActionCommentIssue
	} else if comment.Type == issues_model.CommentTypeCode {
		act = activities_model.ActionCommentIssue
	} else if comment.Type == issues_model.CommentTypePullRequestPush {
		act = 0
	}

	if err := mailer.MailParticipantsComment(ctx, comment, act, issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := mailer.MailParticipants(ctx, issue, issue.Poster, activities_model.ActionCreateIssue, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	var actionType activities_model.ActionType
	if issue.IsPull {
		if isClosed {
			actionType = activities_model.ActionClosePullRequest
		} else {
			actionType = activities_model.ActionReopenPullRequest
		}
	} else {
		if isClosed {
			actionType = activities_model.ActionCloseIssue
		} else {
			actionType = activities_model.ActionReopenIssue
		}
	}

	if err := mailer.MailParticipants(ctx, issue, doer, actionType, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress() {
		if err := mailer.MailParticipants(ctx, issue, doer, activities_model.ActionPullRequestReadyForReview, nil); err != nil {
			log.Error("MailParticipants: %v", err)
		}
	}
}

func (m *mailNotifier) NotifyNewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := mailer.MailParticipants(ctx, pr.Issue, pr.Issue.Poster, activities_model.ActionCreatePullRequest, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	var act activities_model.ActionType
	if comment.Type == issues_model.CommentTypeClose {
		act = activities_model.ActionCloseIssue
	} else if comment.Type == issues_model.CommentTypeReopen {
		act = activities_model.ActionReopenIssue
	} else if comment.Type == issues_model.CommentTypeComment {
		act = activities_model.ActionCommentPull
	}
	if err := mailer.MailParticipantsComment(ctx, comment, act, pr.Issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := mailer.MailMentionsComment(ctx, pr, comment, mentions); err != nil {
		log.Error("MailMentionsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	// mail only sent to added assignees and not self-assignee
	if !removed && doer.ID != assignee.ID && assignee.EmailNotifications() != user_model.EmailNotificationsDisabled {
		ct := fmt.Sprintf("Assigned #%d.", issue.Index)
		if err := mailer.SendIssueAssignedMail(ctx, issue, doer, ct, comment, []*user_model.User{assignee}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to assignee[%d]: %v", issue.ID, assignee.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyPullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if isRequest && doer.ID != reviewer.ID && reviewer.EmailNotifications() != user_model.EmailNotificationsDisabled {
		ct := fmt.Sprintf("Requested to review %s.", issue.HTMLURL())
		if err := mailer.SendIssueAssignedMail(ctx, issue, doer, ct, comment, []*user_model.User{reviewer}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to reviewer[%d]: %v", issue.ID, reviewer.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := mailer.MailParticipants(ctx, pr.Issue, doer, activities_model.ActionMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := mailer.MailParticipants(ctx, pr.Issue, doer, activities_model.ActionAutoMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	var err error
	if err = comment.LoadIssue(ctx); err != nil {
		log.Error("comment.LoadIssue: %v", err)
		return
	}
	if err = comment.Issue.LoadRepo(ctx); err != nil {
		log.Error("comment.Issue.LoadRepo: %v", err)
		return
	}
	if err = comment.Issue.LoadPullRequest(ctx); err != nil {
		log.Error("comment.Issue.LoadPullRequest: %v", err)
		return
	}
	if err = comment.Issue.PullRequest.LoadBaseRepo(ctx); err != nil {
		log.Error("comment.Issue.PullRequest.LoadBaseRepo: %v", err)
		return
	}
	if err := comment.LoadPushCommits(ctx); err != nil {
		log.Error("comment.LoadPushCommits: %v", err)
	}
	m.NotifyCreateIssueComment(ctx, doer, comment.Issue.Repo, comment.Issue, comment, nil)
}

func (m *mailNotifier) NotifyPullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	if err := comment.Review.LoadReviewer(ctx); err != nil {
		log.Error("Error in PullReviewDismiss while loading reviewer for issue[%d], review[%d] and reviewer[%d]: %v", review.Issue.ID, comment.Review.ID, comment.Review.ReviewerID, err)
	}
	if err := mailer.MailParticipantsComment(ctx, comment, activities_model.ActionPullReviewDismissed, review.Issue, nil); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if rel.IsDraft || rel.IsPrerelease {
		return
	}

	mailer.MailNewRelease(ctx, rel)
}

func (m *mailNotifier) NotifyRepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	if err := mailer.SendRepoTransferNotifyMail(ctx, doer, newOwner, repo); err != nil {
		log.Error("SendRepoTransferNotifyMail: %v", err)
	}
}
