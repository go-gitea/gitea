// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mail

import (
	"fmt"

	"code.gitea.io/gitea/models"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
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

func (m *mailNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyCreateIssueComment Issue[%d] #%d in [%d]", issue.ID, issue.Index, issue.RepoID))
	defer finished()

	var act models.ActionType
	if comment.Type == issues_model.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == issues_model.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == issues_model.CommentTypeComment {
		act = models.ActionCommentIssue
	} else if comment.Type == issues_model.CommentTypeCode {
		act = models.ActionCommentIssue
	} else if comment.Type == issues_model.CommentTypePullRequestPush {
		act = 0
	}

	if err := mailer.MailParticipantsComment(ctx, comment, act, issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewIssue(issue *issues_model.Issue, mentions []*user_model.User) {
	if err := mailer.MailParticipants(issue, issue.Poster, models.ActionCreateIssue, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	var actionType models.ActionType
	if issue.IsPull {
		if isClosed {
			actionType = models.ActionClosePullRequest
		} else {
			actionType = models.ActionReopenPullRequest
		}
	} else {
		if isClosed {
			actionType = models.ActionCloseIssue
		} else {
			actionType = models.ActionReopenIssue
		}
	}

	if err := mailer.MailParticipants(issue, doer, actionType, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeTitle(doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress() {
		if err := mailer.MailParticipants(issue, doer, models.ActionPullRequestReadyForReview, nil); err != nil {
			log.Error("MailParticipants: %v", err)
		}
	}
}

func (m *mailNotifier) NotifyNewPullRequest(pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := mailer.MailParticipants(pr.Issue, pr.Issue.Poster, models.ActionCreatePullRequest, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestReview(pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestReview Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	var act models.ActionType
	if comment.Type == issues_model.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == issues_model.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == issues_model.CommentTypeComment {
		act = models.ActionCommentPull
	}
	if err := mailer.MailParticipantsComment(ctx, comment, act, pr.Issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestCodeComment(pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestCodeComment Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	if err := mailer.MailMentionsComment(ctx, pr, comment, mentions); err != nil {
		log.Error("MailMentionsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeAssignee(doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	// mail only sent to added assignees and not self-assignee
	if !removed && doer.ID != assignee.ID && (assignee.EmailNotifications() == user_model.EmailNotificationsEnabled || assignee.EmailNotifications() == user_model.EmailNotificationsOnMention) {
		ct := fmt.Sprintf("Assigned #%d.", issue.Index)
		if err := mailer.SendIssueAssignedMail(issue, doer, ct, comment, []*user_model.User{assignee}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to assignee[%d]: %v", issue.ID, assignee.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyPullReviewRequest(doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if isRequest && doer.ID != reviewer.ID && (reviewer.EmailNotifications() == user_model.EmailNotificationsEnabled || reviewer.EmailNotifications() == user_model.EmailNotificationsOnMention) {
		ct := fmt.Sprintf("Requested to review %s.", issue.HTMLURL())
		if err := mailer.SendIssueAssignedMail(issue, doer, ct, comment, []*user_model.User{reviewer}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to reviewer[%d]: %v", issue.ID, reviewer.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyMergePullRequest(pr *issues_model.PullRequest, doer *user_model.User) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := mailer.MailParticipants(pr.Issue, doer, models.ActionMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestPushCommits(doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestPushCommits Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	var err error
	if err = comment.LoadIssue(); err != nil {
		log.Error("comment.LoadIssue: %v", err)
		return
	}
	if err = comment.Issue.LoadRepo(ctx); err != nil {
		log.Error("comment.Issue.LoadRepo: %v", err)
		return
	}
	if err = comment.Issue.LoadPullRequest(); err != nil {
		log.Error("comment.Issue.LoadPullRequest: %v", err)
		return
	}
	if err = comment.Issue.PullRequest.LoadBaseRepoCtx(ctx); err != nil {
		log.Error("comment.Issue.PullRequest.LoadBaseRepo: %v", err)
		return
	}
	if err := comment.LoadPushCommits(ctx); err != nil {
		log.Error("comment.LoadPushCommits: %v", err)
	}
	m.NotifyCreateIssueComment(doer, comment.Issue.Repo, comment.Issue, comment, nil)
}

func (m *mailNotifier) NotifyPullRevieweDismiss(doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRevieweDismiss Review[%d] in Issue[%d]", review.ID, review.IssueID))
	defer finished()

	if err := mailer.MailParticipantsComment(ctx, comment, models.ActionPullReviewDismissed, review.Issue, nil); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewRelease(rel *models.Release) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyNewRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
	defer finished()

	if err := rel.LoadAttributes(); err != nil {
		log.Error("NotifyNewRelease: %v", err)
		return
	}

	if rel.IsDraft || rel.IsPrerelease {
		return
	}

	mailer.MailNewRelease(ctx, rel)
}

func (m *mailNotifier) NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *repo_model.Repository) {
	if err := mailer.SendRepoTransferNotifyMail(doer, newOwner, repo); err != nil {
		log.Error("NotifyRepoPendingTransfer: %v", err)
	}
}
