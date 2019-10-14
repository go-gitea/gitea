// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

func mailSubject(issue *models.Issue) string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.FullName(), issue.Title, issue.Index)
}

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(issue *models.Issue, doer *models.User, content string, comment *models.Comment, mentions []string) error {
	if !setting.Service.EnableNotifyMail {
		return nil
	}

	watchers, err := models.GetWatchers(issue.RepoID)
	if err != nil {
		return fmt.Errorf("getWatchers [repo_id: %d]: %v", issue.RepoID, err)
	}
	participants, err := models.GetParticipantsByIssueID(issue.ID)
	if err != nil {
		return fmt.Errorf("getParticipantsByIssueID [issue_id: %d]: %v", issue.ID, err)
	}

	// In case the issue poster is not watching the repository and is active,
	// even if we have duplicated in watchers, can be safely filtered out.
	err = issue.LoadPoster()
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", issue.PosterID, err)
	}
	if issue.PosterID != doer.ID && issue.Poster.IsActive && !issue.Poster.ProhibitLogin {
		participants = append(participants, issue.Poster)
	}

	// Assignees must receive any communications
	assignees, err := models.GetAssigneesByIssue(issue)
	if err != nil {
		return err
	}

	for _, assignee := range assignees {
		if assignee.ID != doer.ID {
			participants = append(participants, assignee)
		}
	}

	tos := make([]string, 0, len(watchers)) // List of email addresses.
	names := make([]string, 0, len(watchers))
	for i := range watchers {
		if watchers[i].UserID == doer.ID {
			continue
		}

		to, err := models.GetUserByID(watchers[i].UserID)
		if err != nil {
			return fmt.Errorf("GetUserByID [%d]: %v", watchers[i].UserID, err)
		}
		if to.IsOrganization() || to.EmailNotifications() != models.EmailNotificationsEnabled {
			continue
		}

		tos = append(tos, to.Email)
		names = append(names, to.Name)
	}
	for i := range participants {
		if participants[i].ID == doer.ID ||
			com.IsSliceContainsStr(names, participants[i].Name) ||
			participants[i].EmailNotifications() != models.EmailNotificationsEnabled {
			continue
		}

		tos = append(tos, participants[i].Email)
		names = append(names, participants[i].Name)
	}

	if err := issue.LoadRepo(); err != nil {
		return err
	}

	for _, to := range tos {
		SendIssueCommentMail(issue, doer, content, comment, []string{to})
	}

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}

	emails := models.GetUserEmailsByNames(tos)

	for _, to := range emails {
		SendIssueMentionMail(issue, doer, content, comment, []string{to})
	}

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func MailParticipants(issue *models.Issue, doer *models.User, opType models.ActionType) error {
	return mailParticipants(models.DefaultDBContext(), issue, doer, opType)
}

func mailParticipants(ctx models.DBContext, issue *models.Issue, doer *models.User, opType models.ActionType) (err error) {
	rawMentions := references.FindAllMentionsMarkdown(issue.Content)
	userMentions, err := issue.ResolveMentionsByVisibility(ctx, doer, rawMentions)
	if err != nil {
		return fmt.Errorf("ResolveMentionsByVisibility [%d]: %v", issue.ID, err)
	}
	if err = models.UpdateIssueMentions(ctx, issue.ID, userMentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}
	mentions := make([]string, len(userMentions))
	for i, u := range userMentions {
		mentions[i] = u.LowerName
	}

	if len(issue.Content) > 0 {
		if err = mailIssueCommentToParticipants(issue, doer, issue.Content, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	switch opType {
	case models.ActionCreateIssue, models.ActionCreatePullRequest:
		if len(issue.Content) == 0 {
			ct := fmt.Sprintf("Created #%d.", issue.Index)
			if err = mailIssueCommentToParticipants(issue, doer, ct, nil, mentions); err != nil {
				log.Error("mailIssueCommentToParticipants: %v", err)
			}
		}
	case models.ActionCloseIssue, models.ActionClosePullRequest:
		ct := fmt.Sprintf("Closed #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(issue, doer, ct, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	case models.ActionReopenIssue, models.ActionReopenPullRequest:
		ct := fmt.Sprintf("Reopened #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(issue, doer, ct, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	return nil
}
