// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"context"
	"fmt"

    actions_model "code.gitea.io/gitea/models/actions"
	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

type mailNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &mailNotifier{}

// NewNotifier create a new mailNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &mailNotifier{}
}

func (m *mailNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
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

	if err := MailParticipantsComment(ctx, comment, act, issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := MailParticipants(ctx, issue, issue.Poster, activities_model.ActionCreateIssue, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
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

	if err := MailParticipants(ctx, issue, doer, actionType, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress(ctx) {
		if err := MailParticipants(ctx, issue, doer, activities_model.ActionPullRequestReadyForReview, nil); err != nil {
			log.Error("MailParticipants: %v", err)
		}
	}
}

func (m *mailNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := MailParticipants(ctx, pr.Issue, pr.Issue.Poster, activities_model.ActionCreatePullRequest, mentions); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	var act activities_model.ActionType
	if comment.Type == issues_model.CommentTypeClose {
		act = activities_model.ActionCloseIssue
	} else if comment.Type == issues_model.CommentTypeReopen {
		act = activities_model.ActionReopenIssue
	} else if comment.Type == issues_model.CommentTypeComment {
		act = activities_model.ActionCommentPull
	}
	if err := MailParticipantsComment(ctx, comment, act, pr.Issue, mentions); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := MailMentionsComment(ctx, pr, comment, mentions); err != nil {
		log.Error("MailMentionsComment: %v", err)
	}
}

func (m *mailNotifier) IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	// mail only sent to added assignees and not self-assignee
	if !removed && doer.ID != assignee.ID && assignee.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
		ct := fmt.Sprintf("Assigned #%d.", issue.Index)
		if err := SendIssueAssignedMail(ctx, issue, doer, ct, comment, []*user_model.User{assignee}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to assignee[%d]: %v", issue.ID, assignee.ID, err)
		}
	}
}

func (m *mailNotifier) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if isRequest && doer.ID != reviewer.ID && reviewer.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
		ct := fmt.Sprintf("Requested to review %s.", issue.HTMLURL())
		if err := SendIssueAssignedMail(ctx, issue, doer, ct, comment, []*user_model.User{reviewer}); err != nil {
			log.Error("Error in SendIssueAssignedMail for issue[%d] to reviewer[%d]: %v", issue.ID, reviewer.ID, err)
		}
	}
}

func (m *mailNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := MailParticipants(ctx, pr.Issue, doer, activities_model.ActionMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := MailParticipants(ctx, pr.Issue, doer, activities_model.ActionAutoMergePullRequest, nil); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) PullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
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
	m.CreateIssueComment(ctx, doer, comment.Issue.Repo, comment.Issue, comment, nil)
}

func (m *mailNotifier) PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	if err := comment.Review.LoadReviewer(ctx); err != nil {
		log.Error("Error in PullReviewDismiss while loading reviewer for issue[%d], review[%d] and reviewer[%d]: %v", review.Issue.ID, comment.Review.ID, comment.Review.ReviewerID, err)
	}
	if err := MailParticipantsComment(ctx, comment, activities_model.ActionPullReviewDismissed, review.Issue, nil); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if rel.IsDraft || rel.IsPrerelease {
		return
	}

	MailNewRelease(ctx, rel)
}

func (m *mailNotifier) RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	if err := SendRepoTransferNotifyMail(ctx, doer, newOwner, repo); err != nil {
		log.Error("SendRepoTransferNotifyMail: %v", err)
	}
}

func (m *mailNotifier) ActionRunFinished(ctx context.Context, run *actions_model.ActionRun) {
    // Check status first to avoid unnecessary processing
    if run.Status != actions_model.StatusSuccess && run.Status != actions_model.StatusFailure {
        return
    }

    // Load required attributes after status check
    if err := run.LoadAttributes(ctx); err != nil {
        log.Error("LoadAttributes: %v", err)
        return
    }

    subject := fmt.Sprintf("[%s] Workflow run %s: %s", 
        run.Repo.FullName(), 
        run.WorkflowName, 
        run.Status,
    )

    // Safely handle short commit SHA
    commitSHA := run.CommitSHA
    if len(commitSHA) > 7 {
        commitSHA = commitSHA[:7]
    }

    body := fmt.Sprintf(`Workflow "%s" run #%d has completed with status: %s

Repository: %s
Branch: %s
Commit: %s
Triggered by: %s

View the run details here: %s`,
        run.WorkflowName,
        run.Index,
        run.Status,
        run.Repo.FullName(),
        run.RefName,
        commitSHA,
        run.TriggerUser.Name,
        run.HTMLURL(),
    )

    // Send to repo owner if notifications enabled and email present
    if run.Repo.Owner.Email != "" &&
        run.Repo.Owner.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
        if err := SendMail(ctx, []string{run.Repo.Owner.Email}, subject, body); err != nil {
            log.Error("Failed to send email to repo owner %s: %v", run.Repo.Owner.Email, err)
        }
    }

    // Send to commit author if different from trigger user and notifications enabled
    if run.TriggerUser.ID != run.CommitAuthor.ID &&
        run.CommitAuthor.Email != "" &&
        run.CommitAuthor.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
        if err := SendMail(ctx, []string{run.CommitAuthor.Email}, subject, body); err != nil {
            log.Error("Failed to send email to commit author %s: %v", run.CommitAuthor.Email, err)
        }
    }
}
