// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"context"
	"fmt"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const MailBatchSize = 100 // batch size used in mailIssueCommentBatch

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers (except for WIP pull requests) and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(ctx context.Context, comment *mailComment, mentions []*user_model.User) error {
	// Required by the mail composer; make sure to load these before calling the async function
	if err := comment.Issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("LoadRepo: %w", err)
	}
	if err := comment.Issue.LoadPoster(ctx); err != nil {
		return fmt.Errorf("LoadPoster: %w", err)
	}
	if err := comment.Issue.LoadPullRequest(ctx); err != nil {
		return fmt.Errorf("LoadPullRequest: %w", err)
	}

	// Enough room to avoid reallocations
	unfiltered := make([]int64, 1, 64)

	// =========== Original poster ===========
	unfiltered[0] = comment.Issue.PosterID

	// =========== Assignees ===========
	ids, err := issues_model.GetAssigneeIDsByIssue(ctx, comment.Issue.ID)
	if err != nil {
		return fmt.Errorf("GetAssigneeIDsByIssue(%d): %w", comment.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Participants (i.e. commenters, reviewers) ===========
	ids, err = issues_model.GetParticipantsIDsByIssueID(ctx, comment.Issue.ID)
	if err != nil {
		return fmt.Errorf("GetParticipantsIDsByIssueID(%d): %w", comment.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Issue watchers ===========
	ids, err = issues_model.GetIssueWatchersIDs(ctx, comment.Issue.ID, true)
	if err != nil {
		return fmt.Errorf("GetIssueWatchersIDs(%d): %w", comment.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Repo watchers ===========
	// Make repo watchers last, since it's likely the list with the most users
	if !(comment.Issue.IsPull && comment.Issue.PullRequest.IsWorkInProgress(ctx) && comment.ActionType != activities_model.ActionCreatePullRequest) {
		ids, err = repo_model.GetRepoWatchersIDs(ctx, comment.Issue.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepoWatchersIDs(%d): %w", comment.Issue.RepoID, err)
		}
		unfiltered = append(ids, unfiltered...)
	}

	visited := make(container.Set[int64], len(unfiltered)+len(mentions)+1)

	// Avoid mailing the doer
	if comment.Doer.EmailNotificationsPreference != user_model.EmailNotificationsAndYourOwn && !comment.ForceDoerNotification {
		visited.Add(comment.Doer.ID)
	}

	// =========== Mentions ===========
	if err = mailIssueCommentBatch(ctx, comment, mentions, visited, true); err != nil {
		return fmt.Errorf("mailIssueCommentBatch() mentions: %w", err)
	}

	// Avoid mailing explicit unwatched
	ids, err = issues_model.GetIssueWatchersIDs(ctx, comment.Issue.ID, false)
	if err != nil {
		return fmt.Errorf("GetIssueWatchersIDs(%d): %w", comment.Issue.ID, err)
	}
	visited.AddMultiple(ids...)

	unfilteredUsers, err := user_model.GetMailableUsersByIDs(ctx, unfiltered, false)
	if err != nil {
		return err
	}
	if err = mailIssueCommentBatch(ctx, comment, unfilteredUsers, visited, false); err != nil {
		return fmt.Errorf("mailIssueCommentBatch(): %w", err)
	}

	return nil
}

func mailIssueCommentBatch(ctx context.Context, comment *mailComment, users []*user_model.User, visited container.Set[int64], fromMention bool) error {
	checkUnit := unit.TypeIssues
	if comment.Issue.IsPull {
		checkUnit = unit.TypePullRequests
	}

	langMap := make(map[string][]*user_model.User)
	for _, user := range users {
		if !user.IsActive {
			// Exclude deactivated users
			continue
		}
		// At this point we exclude:
		// user that don't have all mails enabled or users only get mail on mention and this is one ...
		if !(user.EmailNotificationsPreference == user_model.EmailNotificationsEnabled ||
			user.EmailNotificationsPreference == user_model.EmailNotificationsAndYourOwn ||
			fromMention && user.EmailNotificationsPreference == user_model.EmailNotificationsOnMention) {
			continue
		}

		// if we have already visited this user we exclude them
		if !visited.Add(user.ID) {
			continue
		}

		// test if this user is allowed to see the issue/pull
		if !access_model.CheckRepoUnitUser(ctx, comment.Issue.Repo, user, checkUnit) {
			continue
		}

		langMap[user.Language] = append(langMap[user.Language], user)
	}

	for lang, receivers := range langMap {
		// because we know that the len(receivers) > 0 and we don't care about the order particularly
		// working backwards from the last (possibly) incomplete batch. If len(receivers) can be 0 this
		// starting condition will need to be changed slightly
		for i := ((len(receivers) - 1) / MailBatchSize) * MailBatchSize; i >= 0; i -= MailBatchSize {
			msgs, err := composeIssueCommentMessages(ctx, comment, lang, receivers[i:], fromMention, "issue comments")
			if err != nil {
				return err
			}
			SendAsync(msgs...)
			receivers = receivers[:i]
		}
	}

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func MailParticipants(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, opType activities_model.ActionType, mentions []*user_model.User) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	content := issue.Content
	if opType == activities_model.ActionCloseIssue || opType == activities_model.ActionClosePullRequest ||
		opType == activities_model.ActionReopenIssue || opType == activities_model.ActionReopenPullRequest ||
		opType == activities_model.ActionMergePullRequest || opType == activities_model.ActionAutoMergePullRequest {
		content = ""
	}
	forceDoerNotification := opType == activities_model.ActionAutoMergePullRequest
	if err := mailIssueCommentToParticipants(ctx,
		&mailComment{
			Issue:                 issue,
			Doer:                  doer,
			ActionType:            opType,
			Content:               content,
			Comment:               nil,
			ForceDoerNotification: forceDoerNotification,
		}, mentions); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}

// SendIssueAssignedMail composes and sends issue assigned email
func SendIssueAssignedMail(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, content string, comment *issues_model.Comment, recipients []*user_model.User) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("Unable to load repo [%d] for issue #%d [%d]. Error: %v", issue.RepoID, issue.Index, issue.ID, err)
		return err
	}

	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		if !user.IsActive {
			// don't send emails to inactive users
			continue
		}
		langMap[user.Language] = append(langMap[user.Language], user)
	}

	for lang, tos := range langMap {
		msgs, err := composeIssueCommentMessages(ctx, &mailComment{
			Issue:      issue,
			Doer:       doer,
			ActionType: activities_model.ActionType(0),
			Content:    content,
			Comment:    comment,
		}, lang, tos, false, "issue assigned")
		if err != nil {
			return err
		}
		SendAsync(msgs...)
	}
	return nil
}
