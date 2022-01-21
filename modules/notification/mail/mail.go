// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mail

import (
	"fmt"

	"code.gitea.io/gitea/models"
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
	issue *models.Issue, comment *models.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyCreateIssueComment Issue[%d] #%d in [%d]", issue.ID, issue.Index, issue.RepoID))
	defer finished()

	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentIssue
	} else if comment.Type == models.CommentTypeCode {
		act = models.ActionCommentIssue
	} else if comment.Type == models.CommentTypePullRequestPush {
		act = 0
	}

	if err := mailer.MailParticipantsComment(ctx, comment, act, issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewIssue(issue *models.Issue, mentions []*user_model.User) {
	if err := mailer.MailParticipants(issue, issue.Poster, models.ActionCreateIssue, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
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

func (m *mailNotifier) NotifyIssueChangeTitle(doer *user_model.User, issue *models.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && models.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress() {
		if err := mailer.MailParticipants(issue, doer, models.ActionPullRequestReadyForReview, nil); err != nil {
			log.Error("MailParticipants: %v", err)
		}
	}
}

func (m *mailNotifier) NotifyNewPullRequest(pr *models.PullRequest, mentions []*user_model.User) {
	if err := mailer.MailParticipants(pr.Issue, pr.Issue.Poster, models.ActionCreatePullRequest, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, comment *models.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestReview Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentPull
	}
	if err := mailer.MailParticipantsComment(ctx, comment, act, pr.Issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestCodeComment(pr *models.PullRequest, comment *models.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestCodeComment Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	if err := mailer.MailMentionsComment(ctx, pr, comment, mentions); err != nil {
		log.Error("MailMentionsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeAssignee(doer *user_model.User, issue *models.Issue, assignee *user_model.User, removed bool, comment *models.Comment) {
	// mail only sent to added assignees and not self-assignee
	if !removed && doer.ID != assignee.ID && assignee.EmailNotifications() == user_model.EmailNotificationsEnabled {
		ct := fmt.Sprintf("Assigned #%d.", issue.Index)
		if err := mailer.SendIssueAssignedMail(issue, doer, ct, comment, []*user_model.User{assignee}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to assignee[%d]: %v", issue.ID, assignee.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyPullReviewRequest(doer *user_model.User, issue *models.Issue, reviewer *user_model.User, isRequest bool, comment *models.Comment) {
	if isRequest && doer.ID != reviewer.ID && reviewer.EmailNotifications() == user_model.EmailNotificationsEnabled {
		ct := fmt.Sprintf("Requested to review %s.", issue.HTMLURL())
		if err := mailer.SendIssueAssignedMail(issue, doer, ct, comment, []*user_model.User{reviewer}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to reviewer[%d]: %v", issue.ID, reviewer.ID, err)
		}
	}
}

func (m *mailNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *user_model.User) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := mailer.MailParticipants(pr.Issue, doer, models.ActionMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestPushCommits(doer *user_model.User, pr *models.PullRequest, comment *models.Comment) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mailNotifier.NotifyPullRequestPushCommits Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	var err error
	if err = comment.LoadIssue(); err != nil {
		log.Error("comment.LoadIssue: %v", err)
		return
	}
	if err = comment.Issue.LoadRepo(); err != nil {
		log.Error("comment.Issue.LoadRepo: %v", err)
		return
	}
	if err = comment.Issue.LoadPullRequest(); err != nil {
		log.Error("comment.Issue.LoadPullRequest: %v", err)
		return
	}
	if err = comment.Issue.PullRequest.LoadBaseRepo(); err != nil {
		log.Error("comment.Issue.PullRequest.LoadBaseRepo: %v", err)
		return
	}
	if err := comment.LoadPushCommits(ctx); err != nil {
		log.Error("comment.LoadPushCommits: %v", err)
	}
	m.NotifyCreateIssueComment(doer, comment.Issue.Repo, comment.Issue, comment, nil)
}

func (m *mailNotifier) NotifyPullRevieweDismiss(doer *user_model.User, review *models.Review, comment *models.Comment) {
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
